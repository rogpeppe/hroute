package hroute

import (
	"net/http"
)

// NotFound implements
type NotFound struct{}

// ServeRoute implements RouteHandler.ServeRoute by calling http.NotFound.
func (h NotFound) ServeRoute(w http.ResponseWriter, req *http.Request, _ Params) {
	http.NotFound(w, req)
}

type MethodNotAllowed struct{}

func (h MethodNotAllowed) ServeRoute(w http.ResponseWriter, req *http.Request, _ Params) {
	http.Error(w,
		http.StatusText(http.StatusMethodNotAllowed),
		http.StatusMethodNotAllowed,
	)
}

type Redirect struct {
	Path string
	Code int
}

func (r Redirect) ServeRoute(w http.ResponseWriter, req *http.Request, _ Params) {
	http.Redirect(w, req, r.Path, r.Code)
}

type RouteHandlerFunc func(http.ResponseWriter, *http.Request, Params)

func (r RouteHandlerFunc) ServeRoute(w http.ResponseWriter, req *http.Request, p Params) {
	r(w, req, p)
}
