package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/adnanh/webhook/helpers"
	"github.com/adnanh/webhook/hooks"

	"github.com/go-martini/martini"

	l4g "code.google.com/p/log4go"
)

const (
	version string = "1.0.4"
)

var (
	webhooks      *hooks.Hooks
	appStart      time.Time
	ip            = flag.String("ip", "", "ip the webhook server should listen on")
	port          = flag.Int("port", 9000, "port the webhook server should listen on")
	hooksFilename = flag.String("hooks", "hooks.json", "path to the json file containing defined hooks the webhook should serve")
	logFilename   = flag.String("log", "webhook.log", "path to the log file")
)

func init() {
	flag.Parse()

	fileLogWriter := l4g.NewFileLogWriter(*logFilename, false)
	fileLogWriter.SetRotateDaily(false)

	martini.Env = "production"

	l4g.AddFilter("file", l4g.FINE, fileLogWriter)
}

func main() {
	appStart = time.Now()
	var e error

	webhooks, e = hooks.New(*hooksFilename)

	if e != nil {
		l4g.Warn("Error occurred while loading hooks from %s: %s", *hooksFilename, e)
	}

	web := martini.Classic()

	web.Get("/", rootHandler)
	web.Get("/hook/:id", hookHandler)
	web.Post("/hook/:id", hookHandler)

	l4g.Info("Starting webhook %s with %d hook(s) on %s:%d", version, webhooks.Count(), *ip, *port)

	web.RunOnAddr(fmt.Sprintf("%s:%d", *ip, *port))
}

func rootHandler() string {
	return fmt.Sprintf("webhook %s running for %s serving %d hook(s)\n", version, time.Since(appStart).String(), webhooks.Count())
}

func jsonHandler(id string, body []byte, signature string, payload interface{}) {
	if hook := webhooks.Match(id, payload); hook != nil {
		if hook.Secret != "" {
			if signature == "" {
				l4g.Error("Hook %s got matched and contains the secret, but the request didn't contain any signature.", hook.ID)
				return
			}

			if expectedMAC, ok := helpers.CheckPayloadSignature(body, hook.Secret, signature); !ok {
				l4g.Error("Hook %s got matched and contains the secret, but the request contained invalid signature. Expected %s, got %s.", hook.ID, expectedMAC, signature)
				return
			}
		}

		cmd := exec.Command(hook.Command)
		cmd.Args = hook.ParseJSONArgs(payload)
		cmd.Dir = hook.Cwd
		out, err := cmd.Output()
		l4g.Info("Hook %s triggered successfully! Command output:\n%s\n%+v", hook.ID, out, err)
	}
}

func formHandler(id string, formValues url.Values) {
	if hook := webhooks.Match(id, helpers.FormValuesToMap(formValues)); hook != nil {
		cmd := exec.Command(hook.Command)
		cmd.Args = hook.ParseFormArgs(formValues)
		cmd.Dir = hook.Cwd
		out, err := cmd.Output()
		l4g.Info("Hook %s triggered successfully! Command output:\n%s\n%+v", hook.ID, out, err)
	}
}

func hookHandler(req *http.Request, params martini.Params) string {
	if req.Header.Get("Content-Type") == "application/json" {
		defer req.Body.Close()

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			l4g.Warn("Error occurred while trying to read the request body: %s", err)
		}

		payloadJSON := make(map[string]interface{})

		decoder := json.NewDecoder(strings.NewReader(string(body)))
		decoder.UseNumber()

		err = decoder.Decode(&payloadJSON)

		if err != nil {
			l4g.Warn("Error occurred while trying to parse the payload as JSON: %s", err)
		}

		payloadSignature := ""

		if strings.Contains(req.Header.Get("User-Agent"), "GitHub-Hookshot") {
			if len(req.Header.Get("X-Hub-Signature")) > 5 {
				payloadSignature = req.Header.Get("X-Hub-Signature")[5:]
			}
		}
		
		go jsonHandler(params["id"], body, payloadSignature, payloadJSON)
	} else {
		req.ParseForm()
		go formHandler(params["id"], req.Form)
	}

	return "Got it, thanks. :-)"
}
