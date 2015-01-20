package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/adnanh/webhook/hooks"

	"github.com/go-martini/martini"

	"crypto/hmac"
	"crypto/sha1"

	l4g "code.google.com/p/log4go"
)

const (
	version string = "1.0.2"
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

func hookHandler(req *http.Request, params martini.Params) string {
	defer req.Body.Close()
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		l4g.Warn("Error occurred while trying to read the request body: %s", err)
	}

	payloadJSON := make(map[string]interface{})

	if req.Header.Get("Content-Type") == "application/json" {
		decoder := json.NewDecoder(strings.NewReader(string(body)))
		decoder.UseNumber()

		err := decoder.Decode(&payloadJSON)

		if err != nil {
			l4g.Warn("Error occurred while trying to parse the payload as JSON: %s", err)
		}
	}

	go func(id string, body []byte, signature string, params interface{}) {
		if hook := webhooks.Match(id, params); hook != nil {
			if hook.Secret != "" {
				if signature == "" {
					l4g.Error("Hook %s got matched, but the request contained invalid signature.", hook.ID)
					return
				}

				mac := hmac.New(sha1.New, []byte(hook.Secret))
				mac.Write(body)
				expectedMAC := hex.EncodeToString(mac.Sum(nil))

				if !hmac.Equal([]byte(signature), []byte(expectedMAC)) {
					l4g.Error("Hook %s got matched, but the request contained invalid signature. Expected %s, got %s.", hook.ID, expectedMAC, signature)
					return
				}
			}

			cmd := exec.Command(hook.Command, "", "", hook.Cwd)
			out, _ := cmd.Output()
			l4g.Info("Hook %s triggered successfully! Command output:\n%s", hook.ID, out)
		}
	}(params["id"], body, req.Header.Get("X-Hub-Signature")[5:], payloadJSON)

	return "Got it, thanks. :-)"
}
