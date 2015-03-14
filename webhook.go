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

	"github.com/adnanh/webhook/helpers"
	"github.com/adnanh/webhook/hook"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"

	fsnotify "gopkg.in/fsnotify.v1"
)

const (
	version = "2.2.0"
)

var (
	ip            = flag.String("ip", "", "ip the webhook should serve hooks on")
	port          = flag.Int("port", 9000, "port the webhook should serve hooks on")
	verbose       = flag.Bool("verbose", false, "show verbose output")
	hotReload     = flag.Bool("hotreload", false, "watch hooks file for changes and reload them automatically")
	hooksFilePath = flag.String("hooks", "hooks.json", "path to the json file containing defined hooks the webhook should serve")
	secure        = flag.Bool("secure", false, "use HTTPS instead of HTTP")
	cert          = flag.String("cert", "cert.pem", "path to the HTTPS certificate pem file")
	key           = flag.String("key", "key.pem", "path to the HTTPS certificate private key pem file")

	watcher *fsnotify.Watcher

	hooks hook.Hooks
)

func init() {
	hooks = hook.Hooks{}

	flag.Parse()

	log.SetPrefix("[webhook] ")
	log.SetFlags(log.Ldate | log.Ltime)

	if !*verbose {
		log.SetOutput(ioutil.Discard)
	}

	log.Println("version " + version + " starting")

	// load and parse hooks
	log.Printf("attempting to load hooks from %s\n", *hooksFilePath)

	err := hooks.LoadFromFile(*hooksFilePath)

	if err != nil {
		log.Printf("couldn't load hooks from file! %+v\n", err)
	} else {
		log.Printf("loaded %d hook(s) from file\n", len(hooks))

		for _, hook := range hooks {
			log.Printf("\t> %s\n", hook.ID)
		}
	}
}

func main() {
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

	l := log.New(os.Stdout, "[webhook] ", log.Ldate|log.Ltime)

	negroniLogger := &negroni.Logger{l}

	negroniRecovery := &negroni.Recovery{
		Logger:     l,
		PrintStack: true,
		StackAll:   false,
		StackSize:  1024 * 8,
	}

	n := negroni.New(negroniRecovery, negroniLogger)

	router := mux.NewRouter()
	router.HandleFunc("/hooks/{id}", hookHandler)

	n.UseHandler(router)

	if *secure {
		log.Printf("starting secure (https) webhook on %s:%d", *ip, *port)
		log.Fatal(http.ListenAndServeTLS(fmt.Sprintf("%s:%d", *ip, *port), *cert, *key, n))
	} else {
		log.Printf("starting insecure (http) webhook on %s:%d", *ip, *port)
		log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", *ip, *port), n))
	}

}

func hookHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	hook := hooks.Match(id)

	if hook != nil {
		log.Printf("%s got matched\n", id)

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("error reading the request body. %+v\n", err)
		}

		// parse headers
		headers := helpers.ValuesToMap(r.Header)

		// parse query variables
		query := helpers.ValuesToMap(r.URL.Query())

		// parse body
		var payload map[string]interface{}

		contentType := r.Header.Get("Content-Type")

		if contentType == "application/json" {
			decoder := json.NewDecoder(strings.NewReader(string(body)))
			decoder.UseNumber()

			err := decoder.Decode(&payload)

			if err != nil {
				log.Printf("error parsing JSON payload %+v\n", err)
			}
		} else if contentType == "application/x-www-form-urlencoded" {
			fd, err := url.ParseQuery(string(body))
			if err != nil {
				log.Printf("error parsing form payload %+v\n", err)
			} else {
				payload = helpers.ValuesToMap(fd)
			}
		}

		// handle hook
		go handleHook(hook, &headers, &query, &payload, &body)

		// say thanks
		fmt.Fprintf(w, "Thanks.")
	} else {
		fmt.Fprintf(w, "Hook not found.")
	}
}

func handleHook(hook *hook.Hook, headers, query, payload *map[string]interface{}, body *[]byte) {
	if hook.TriggerRule == nil || hook.TriggerRule != nil && hook.TriggerRule.Evaluate(headers, query, payload, body) {
		log.Printf("%s hook triggered successfully\n", hook.ID)

		cmd := exec.Command(hook.ExecuteCommand)
		cmd.Args = hook.ExtractCommandArguments(headers, query, payload)
		cmd.Dir = hook.CommandWorkingDirectory

		log.Printf("executing %s (%s) with arguments %s using %s as cwd\n", hook.ExecuteCommand, cmd.Path, cmd.Args, cmd.Dir)

		out, err := cmd.Output()

		log.Printf("stdout: %s\n", out)

		if err != nil {
			log.Printf("stderr: %+v\n", err)
		}
		log.Printf("finished handling %s\n", hook.ID)
	} else {
		log.Printf("%s hook did not get triggered\n", hook.ID)
	}
}

func watchForFileChange() {
	for {
		select {
		case event := <-(*watcher).Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				log.Println("hooks file modified")

				newHooks := hook.Hooks{}

				// parse and swap
				log.Printf("attempting to reload hooks from %s\n", *hooksFilePath)

				err := newHooks.LoadFromFile(*hooksFilePath)

				if err != nil {
					log.Printf("couldn't load hooks from file! %+v\n", err)
				} else {
					log.Printf("loaded %d hook(s) from file\n", len(hooks))

					for _, hook := range hooks {
						log.Printf("\t> %s\n", hook.ID)
					}

					hooks = newHooks
				}
			}
		case err := <-(*watcher).Errors:
			log.Println("watcher error:", err)
		}
	}
}
