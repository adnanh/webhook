package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/adnanh/webhook/hooks"

	"github.com/go-martini/martini"
)

const (
	version string = "1.0.0"
	ip      string = ""
	port    int    = 9000
)

var (
	webhooks *hooks.Hooks
	appStart time.Time
)

func main() {
	appStart = time.Now()
	var e error

	webhooks, e = hooks.New("hooks.json")

	if e != nil {
		fmt.Printf("Error while loading hooks from hooks.json:\n\t>>> %s\n", e)
	}

	web := martini.Classic()

	web.Get("/", rootHandler)
	web.Get("/hook/:id", hookHandler)
	web.Post("/hook/:id", hookHandler)

	fmt.Printf("Starting go-webhook with %d hook(s)\n\n", webhooks.Count())

	web.RunOnAddr(fmt.Sprintf("%s:%d", ip, port))
}

func rootHandler() string {
	return fmt.Sprintf("go-webhook %s running for %s serving %d hook(s)\n%+v", version, time.Since(appStart).String(), webhooks.Count(), webhooks)
}

func hookHandler(req *http.Request, params martini.Params) string {

	decoder := json.NewDecoder(req.Body)
	decoder.UseNumber()

	p := make(map[string]interface{})

	err := decoder.Decode(&p)

	if err != nil {
		return "Error occurred while parsing the payload :-("
	}

	go func(id string, params interface{}) {
		if hook := webhooks.Match(id, params); hook != nil {
			cmd := exec.Command(hook.Command, "", "", hook.Cwd)
			out, _ := cmd.Output()
			fmt.Printf("Command output for %v >>> %s\n", hook, out)
		}
	}(params["id"], p)
	return "Got it, thanks. :-)"
}
