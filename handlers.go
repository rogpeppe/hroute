package hroute

import (
	"net/http"
)

type NotFound struct{}

func (h NotFound) ServeHTTP(w http.ResponseWriter, req *http.Request, _ Params) {
	http.NotFound(w, req)
}

type MethodNotAllowed struct{}

func (h MethodNotAllowed) ServeHTTP(w http.ResponseWriter, req *http.Request, _ Params) {
	http.Error(w,
		http.StatusText(http.StatusMethodNotAllowed),
		http.StatusMethodNotAllowed,
	)
}

type Redirect struct {
	Path string
	Code int
}

func (r Redirect) ServeHTTP(w http.ResponseWriter, req *http.Request, _ Params) {
	http.Redirect(w, req, r.Path, r.Code)
}
