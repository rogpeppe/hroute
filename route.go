package hroute

import (
	"net/http"
	"strings"
)

// Router represents an HTTP router. Its exported variables should not be
// changed while HTTP requests are being served.
type Router struct {
	root *node

	// NotFoundHandler is the handler used when no matching route is found.
	// If it is nil, NotFound{} is used.
	NotFound Handler

	// MethodNotAllowedHandler is the handler used when a handler
	// cannot be found for a given method but there is a handler
	// for the requested path. If it is nil, MethodNotAllowed{} will be
	// used.
	MethodNotAllowed Handler

	// When Panic is not nil, panics in handlers will be
	// recovered and PanicHandler will be called with the HTTP
	// handler parameters, the Handler responsible for the panic and
	// any parameters it was passed, and the recovered panic value.
	//
	// It should be used to generate a error page and return the
	// http error code 500 (Internal Server Error). The handler can
	// be used to keep your server from crashing because of
	// unrecovered panics.
	Panic func(w http.ResponseWriter, req *http.Request, h http.Handler, p Params, err interface{})
}

// Param holds a path parameter that represents the value of
// a wildcard parameter.
type Param struct {
	// Key holds the key of the parameter.
	Key   string

	// Value holds its value. When the wildcard is a "*",
	// the value will always hold a leading slash.
	Value string
}

// Params represents the values for a set of wildcard parameters.
// There will only be one instance of any given key.
type Params []Param

// RouteHandler is the interface implemented by hroute HTTP handlers.
// See HTTPHandler for an adaptor that will put the parameters
// into the request context (only available on Go 1.7 and later).
type RouteHandler interface {
	HandleRoute(http.ResponseWriter, *http.Request, Params)
}

// New returns a new Router.
// Path auto-correction, including trailing slashes, is enabled by default.
func New() *Router {
	return &Router{
		root: &node{
			path: "/",
		},
		NotFound:         NotFound{},
		MethodNotAllowed: MethodNotAllowed{},
	}
}

// Handle registers the handler for the given pattern and methods.
// If a handler is already registered for the given pattern
// or the pattern is invalid, Handle panics.
//
// It returns the parsed pattern, suitable for recreating the path.
func (r *Router) Handle(method, pattern string, handler RouteHandler) *Pattern {
	pat, err := ParsePattern(pattern)
	if err != nil {
		panic(err)
	}
	r.root.addRoute(pat, methid, h)
	return pat
}

func (r *Router) HandleFunc(method, pattern string, handler func(http.ResponseWriter, *http.Request, Params)) *Pattern {
	return r.Handle(method, pattern, RouteHandlerFunc(handler))
}

// Handler returns the handler to use for the given method and path,
// the pattern associated with the route and parameters
// appropriate for passing to the handler.
func (r *Router) Handler(method, path string) (RouteHandler, *Pattern, Params)

// ServeHTTP implements http.Handler by consulting req.URL.Method
// and req.URL.Path and calling the registered handler that most closely
// matches.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.ServeHTTPSubroute(w, req, req.URL.Path)
}

// Handler looks up the handler for the given method and path
// and returns the found handler and its matched parameters.
// If there is no handler found, it returns zero results.
func (r *Router) Handler(method string, path string) (Handler, Params) {
	h, p, _ := r.root.getValue(method, path)
	return h, p
}

// HandlerToUse returns the handler that will be used to handle
// a request with the given method and path.
// It never returns a nil handler.
// If a handler has not been registered with the given path,
// a value of type NotFound, MethodNotAllowed or Redirect
// will be returned.
func (r *Router) HandlerToUse(method, path string) (RouteHandler, Params) {
	h, p, node := r.root.getValue(method, path)
	if h != nil {
		return h, p
	}
	if node != nil && len(node.handlers) > 0 {
		// There is at least one other handler defined for this path,
		// so don't redirect.
		return r.MethodNotAllowed, Params{}
	}
	if method == "CONNECT" || path == "/" {
		// Can't redirect CONNECT; no need to redirect /.
		return r.NotFound, Params{}
	}
	code := http.StatusMovedPermanently // Permanent redirect, request with GET method
	if method != "GET" {
		// Temporary redirect, request with same method
		// TODO use StatusPermanentRedirect ?
		code = http.StatusTemporaryRedirect
	}
	if cleanPath := CleanPath(path); cleanPath != path {
		return Redirect{
			Path: cleanPath,
			Code: code,
		}, Params{}
	}
	if redirectPath := r.slashRedirect(method, path); redirectPath != "" {
		return Redirect{
			Path: redirectPath,
			Code: code,
		}, Params{}
	}
	return r.NotFound, Params{}
}

// ServeHTTPSubroute is like ServeHTTP except that instead of using
// req.URL.Path to route requests, it uses the given path
// parameter.
func (r *Router) ServeHTTPSubroute(w http.ResponseWriter, req *http.Request, path string) {
	handler, params := r.HandlerToUse(req.Method, path)
	if r.Panic != nil {
		defer r.recover(w, req, handler, params)
	}
	handler.ServeHTTP(w, req, params)
}

func (r *Router) recover(w http.ResponseWriter, req *http.Request, h Handler, p Params) {
	if rcv := recover(); rcv != nil {
		r.Panic(w, req, h, p, rcv)
	}
}

// slashRedirect returns a possible redirected path when the
// given path cannot be found.
func (r *Router) slashRedirect(method, path string) string {
	if strings.HasSuffix(path, "/") {
		path = path[0 : len(path)-1]
	} else {
		path += "/"
	}
	n, _ := r.root.lookup(path)
	if n == nil {
		return ""
	}
	_, ok := n.handlers[method]
	if !ok {
		return ""
	}
	return path
}
