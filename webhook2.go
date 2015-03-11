package main

import (
	"fmt"
	"net/http"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
)

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/hooks/{id}", hookHandler)

	n := negroni.Classic()
	n.UseHandler(router)

	n.Run(":9000")
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
