package hroute

import (
	"net/http"
)

// NotFound is used as the default hander when a route is not
// found in the router.
type NotFound struct{}

// ServeRoute implements Handler.ServeRoute by calling http.NotFound.
func (h NotFound) ServeRoute(w http.ResponseWriter, req *http.Request, _ Params) {
	http.NotFound(w, req)
}

// MethodNotAllowed is used as the default handler
// when an implementation for a method is not found.
type MethodNotAllowed struct{}

// ServeRoute implements Handler.ServeRoute by returning an StatusMethodNotAllowed response.
func (h MethodNotAllowed) ServeRoute(w http.ResponseWriter, req *http.Request, _ Params) {
	http.Error(w,
		http.StatusText(http.StatusMethodNotAllowed),
		http.StatusMethodNotAllowed,
	)
}

// Redirect is used as the handler when the router requires a redirection.
type Redirect struct {
	Path string
	Code int
}

// ServeRoute implements Handler by redirecting to r.Path with the response
// status r.Code.
func (r Redirect) ServeRoute(w http.ResponseWriter, req *http.Request, _ Params) {
	http.Redirect(w, req, r.Path, r.Code)
}

// HandlerFunc implements Handler for a function.
type HandlerFunc func(http.ResponseWriter, *http.Request, Params)

// ServeRoute implements Handler by calling r with the given arguments.
func (r HandlerFunc) ServeRoute(w http.ResponseWriter, req *http.Request, p Params) {
	r(w, req, p)
}
