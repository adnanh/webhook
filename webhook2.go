package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
)

var (
	ip            = flag.String("ip", "", "ip the webhook should serve hooks on")
	port          = flag.Int("port", 9000, "port the webhook should serve hooks on")
	verbose       = flag.Bool("verbose", false, "show verbose output")
	hooksFilePath = flag.String("hooks", "hooks.json", "path to the json file containing defined hooks the webhook should serve")
)

func init() {
	flag.Parse()

	log.SetPrefix("[webhook] ")
	log.SetFlags(log.Ldate | log.Ltime)

	if !*verbose {
		log.SetOutput(ioutil.Discard)
	}

	log.Println("starting")

	// load and parse hooks
	log.Printf("attempting to load hooks from %s\n", *hooksFilePath)

	// set up file watcher
	log.Printf("setting up file watcher for %s\n", *hooksFilePath)
}

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/hooks/{id}", hookHandler)

	n := negroni.Classic()
	n.UseHandler(router)

	n.Run(fmt.Sprintf("%s:%d", *ip, *port))
}

func hookHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// parse headers

	// parse body

	// find hook

	// trigger hook

	// say thanks

	fmt.Fprintf(w, "Thanks. %s %+v %+v %+v", id, vars, r.Header, r.Body)
}
