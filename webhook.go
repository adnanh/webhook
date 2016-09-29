package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/adnanh/webhook/hook"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"

	fsnotify "gopkg.in/fsnotify.v1"
)

const (
	version = "2.5.0"
)

var (
	ip             = flag.String("ip", "0.0.0.0", "ip the webhook should serve hooks on")
	port           = flag.Int("port", 9000, "port the webhook should serve hooks on")
	verbose        = flag.Bool("verbose", false, "show verbose output")
	noPanic        = flag.Bool("nopanic", false, "do not panic if hooks cannot be loaded when webhook is not running in verbose mode")
	hotReload      = flag.Bool("hotreload", false, "watch hooks file for changes and reload them automatically")
	hooksFilePath  = flag.String("hooks", "hooks.json", "path to the json file containing defined hooks the webhook should serve")
	hooksURLPrefix = flag.String("urlprefix", "hooks", "url prefix to use for served hooks (protocol://yourserver:port/PREFIX/:hook-id)")
	secure         = flag.Bool("secure", false, "use HTTPS instead of HTTP")
	cert           = flag.String("cert", "cert.pem", "path to the HTTPS certificate pem file")
	key            = flag.String("key", "key.pem", "path to the HTTPS certificate private key pem file")

	responseHeaders hook.ResponseHeaders

	watcher *fsnotify.Watcher
	signals chan os.Signal

	hooks hook.Hooks
)

func main() {
	hooks = hook.Hooks{}

	flag.Var(&responseHeaders, "header", "response header to return, specified in format name=value, use multiple times to set multiple headers")

	flag.Parse()

	log.SetPrefix("[webhook] ")
	log.SetFlags(log.Ldate | log.Ltime)

	if !*verbose {
		log.SetOutput(ioutil.Discard)
	}

	log.Println("version " + version + " starting")

	// set os signal watcher
	setupSignals()

	// load and parse hooks
	log.Printf("attempting to load hooks from %s\n", *hooksFilePath)

	err := hooks.LoadFromFile(*hooksFilePath)

	if err != nil {
		if !*verbose && !*noPanic {
			log.SetOutput(os.Stdout)
			log.Fatalf("couldn't load any hooks from file! %+v\naborting webhook execution since the -verbose flag is set to false.\nIf, for some reason, you want webhook to start without the hooks, either use -verbose flag, or -nopanic", err)
		}

		log.Printf("couldn't load hooks from file! %+v\n", err)
	} else {
		seenHooksIds := make(map[string]bool)

		log.Printf("found %d hook(s) in file\n", len(hooks))

		for _, hook := range hooks {
			if seenHooksIds[hook.ID] == true {
				log.Fatalf("error: hook with the id %s has already been loaded!\nplease check your hooks file for duplicate hooks ids!\n", hook.ID)
			}
			seenHooksIds[hook.ID] = true
			log.Printf("\tloaded: %s\n", hook.ID)
		}
	}

	if *hotReload {
		// set up file watcher
		log.Printf("setting up file watcher for %s\n", *hooksFilePath)

		var err error

		watcher, err = fsnotify.NewWatcher()
		if err != nil {
			log.Fatal("error creating file watcher instance", err)
		}

		defer watcher.Close()

		go watchForFileChange()

		err = watcher.Add(*hooksFilePath)
		if err != nil {
			log.Fatal("error adding hooks file to the watcher", err)
		}
	}

	l := negroni.NewLogger()
	l.ALogger = log.New(os.Stderr, "[webhook] ", log.Ldate|log.Ltime)

	negroniRecovery := &negroni.Recovery{
		Logger:     l.ALogger,
		PrintStack: true,
		StackAll:   false,
		StackSize:  1024 * 8,
	}

	n := negroni.New(negroniRecovery, l)

	router := mux.NewRouter()

	var hooksURL string

	if *hooksURLPrefix == "" {
		hooksURL = "/{id}"
	} else {
		hooksURL = "/" + *hooksURLPrefix + "/{id}"
	}

	router.HandleFunc(hooksURL, hookHandler)

	n.UseHandler(router)

	if *secure {
		log.Printf("serving hooks on https://%s:%d%s", *ip, *port, hooksURL)
		log.Fatal(http.ListenAndServeTLS(fmt.Sprintf("%s:%d", *ip, *port), *cert, *key, n))
	} else {
		log.Printf("serving hooks on http://%s:%d%s", *ip, *port, hooksURL)
		log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", *ip, *port), n))
	}

}

func hookHandler(w http.ResponseWriter, r *http.Request) {
	for _, responseHeader := range responseHeaders {
		w.Header().Set(responseHeader.Name, responseHeader.Value)
	}

	id := mux.Vars(r)["id"]

	if matchedHook := hooks.Match(id); matchedHook != nil {
		log.Printf("%s got matched\n", id)

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("error reading the request body. %+v\n", err)
		}

		// parse headers
		headers := valuesToMap(r.Header)

		// parse query variables
		query := valuesToMap(r.URL.Query())

		// parse body
		var payload map[string]interface{}

		contentType := r.Header.Get("Content-Type")

		if strings.Contains(contentType, "json") {
			decoder := json.NewDecoder(strings.NewReader(string(body)))
			decoder.UseNumber()

			err := decoder.Decode(&payload)

			if err != nil {
				log.Printf("error parsing JSON payload %+v\n", err)
			}
		} else if strings.Contains(contentType, "form") {
			fd, err := url.ParseQuery(string(body))
			if err != nil {
				log.Printf("error parsing form payload %+v\n", err)
			} else {
				payload = valuesToMap(fd)
			}
		}

		// handle hook
		if err = matchedHook.ParseJSONParameters(&headers, &query, &payload); err != nil {
			msg := fmt.Sprintf("error parsing JSON parameters: %s", err)
			log.Printf(msg)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Unable to parse JSON parameters.")
			return
		}

		var ok bool

		if matchedHook.TriggerRule == nil {
			ok = true
		} else {
			ok, err = matchedHook.TriggerRule.Evaluate(&headers, &query, &payload, &body)
			if err != nil {
				msg := fmt.Sprintf("error evaluating hook: %s", err)
				log.Printf(msg)
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "Error occurred while evaluating hook rules.")
				return
			}
		}

		if ok {
			log.Printf("%s hook triggered successfully\n", matchedHook.ID)

			for _, responseHeader := range matchedHook.ResponseHeaders {
				w.Header().Set(responseHeader.Name, responseHeader.Value)
			}

			if matchedHook.CaptureCommandOutput {
				response, err := handleHook(matchedHook, &headers, &query, &payload, &body)

				if err != nil {
					w.Header().Set("Content-Type", "text/plain; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintf(w, "Error occurred while executing the hook's command. Please check your logs for more details.\n")
				} else {
					fmt.Fprintf(w, response)
				}
			} else {
				go handleHook(matchedHook, &headers, &query, &payload, &body)
				fmt.Fprintf(w, matchedHook.ResponseMessage)
			}
			return
		}

		// if none of the hooks got triggered
		log.Printf("%s got matched, but didn't get triggered because the trigger rules were not satisfied\n", matchedHook.ID)

		fmt.Fprintf(w, "Hook rules were not satisfied.")
	} else {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Hook not found.")
	}
}

func handleHook(h *hook.Hook, headers, query, payload *map[string]interface{}, body *[]byte) (string, error) {
	var err error

	cmd := exec.Command(h.ExecuteCommand)
	cmd.Dir = h.CommandWorkingDirectory

	cmd.Args, err = h.ExtractCommandArguments(headers, query, payload)
	if err != nil {
		log.Printf("error extracting command arguments: %s", err)
	}

	var envs []string
	envs, err = h.ExtractCommandArgumentsForEnv(headers, query, payload)
	if err != nil {
		log.Printf("error extracting command arguments for environment: %s", err)
	}
	cmd.Env = append(os.Environ(), envs...)

	log.Printf("executing %s (%s) with arguments %q and environment %s using %s as cwd\n", h.ExecuteCommand, cmd.Path, cmd.Args, envs, cmd.Dir)

	out, err := cmd.Output()

	log.Printf("command output: %s\n", out)

	if err != nil {
		log.Printf("error occurred: %+v\n", err)
	}

	log.Printf("finished handling %s\n", h.ID)

	return string(out), err
}

func reloadHooks() {
	newHooks := hook.Hooks{}

	// parse and swap
	log.Printf("attempting to reload hooks from %s\n", *hooksFilePath)

	err := newHooks.LoadFromFile(*hooksFilePath)

	if err != nil {
		log.Printf("couldn't load hooks from file! %+v\n", err)
	} else {
		seenHooksIds := make(map[string]bool)

		log.Printf("found %d hook(s) in file\n", len(newHooks))

		for _, hook := range newHooks {
			if seenHooksIds[hook.ID] == true {
				log.Printf("error: hook with the id %s has already been loaded!\nplease check your hooks file for duplicate hooks ids!", hook.ID)
				log.Println("reverting hooks back to the previous configuration")
				return
			}
			seenHooksIds[hook.ID] = true
			log.Printf("\tloaded: %s\n", hook.ID)
		}

		hooks = newHooks
	}
}

func watchForFileChange() {
	for {
		select {
		case event := <-(*watcher).Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				log.Println("hooks file modified")

				reloadHooks()
			}
		case err := <-(*watcher).Errors:
			log.Println("watcher error:", err)
		}
	}
}

// valuesToMap converts map[string][]string to a map[string]string object
func valuesToMap(values map[string][]string) map[string]interface{} {
	ret := make(map[string]interface{})

	for key, value := range values {
		if len(value) > 0 {
			ret[key] = value[0]
		}
	}

	return ret
}
