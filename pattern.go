package hrouter

import (
	"fmt"
	"strings"

	"github.com/julienschmidt/httprouter"
)

// pattern holds a parsed path pattern. Each segment of
// static holds a static segment of the path. Each var
// holds the name of a wildcard variable inside the path
// between two pattern segments.
// If catchAll is true, the last variable is a * pattern
// that matches any number of trailing path elements.
//
// Every place in static corresponding to a wildcard
// variable is empty. The variable for static[i]
// is at vars[i/2].
//
// For example, parsing: /:foo/a/b/c/:e/*c
// would result in:
//
//	pattern{
//		static: {"/", "", "/a/b/c/", "", "/", ""},
//		vars: {"foo", "e", "c"},
//		catchAll: true,
//	}
type pattern struct {
	static   []string
	vars     []string
	catchAll bool
}

func parsePattern(p string) (pattern, error) {
	if httprouter.CleanPath(p) != p {
		return pattern{}, fmt.Errorf("path is not clean")
	}
	var pat pattern
	if !strings.HasPrefix(p, "/") {
		return pattern{}, fmt.Errorf("path must start with /")
	}
	for len(p) > 0 {
		i := strings.IndexAny(p, ":*")
		if i == -1 {
			pat.static = append(pat.static, p)
			return pat, nil
		}
		if i == 0 {
			panic("unexpected empty path segment")
		}
		pat.static = append(pat.static, p[0:i])
		if p[i-1] != '/' {
			return pattern{}, fmt.Errorf("no / before wildcard segment")
		}
		p = p[i:]
		i = strings.Index(p, "/")
		if i == -1 {
			pat.static = append(pat.static, "")
			pat.vars = append(pat.vars, p[1:])
			pat.catchAll = p[0] == '*'
			return pat, nil
		}
		if p[0] == '*' {
			return pattern{}, fmt.Errorf("catch-all route not at end of path")
		}
		v := p[1:i]
		if strings.IndexAny(v, ":*") != -1 {
			return pattern{}, fmt.Errorf("no / before wildcard segment")
		}
		pat.static = append(pat.static, "")
		pat.vars = append(pat.vars, v)
		p = p[i:]
	}
	return pat, nil
}
