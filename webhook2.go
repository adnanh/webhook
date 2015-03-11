package main

import (
	"flag"
	"fmt"
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

	// load and parse hooks

	// set up file watcher
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
