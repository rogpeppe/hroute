package hrouter

import (
	"fmt"
	"strings"

	"gopkg.in/errgo.v1"

	"github.com/julienschmidt/httprouter"
)

// Pattern holds a parsed path pattern.
type Pattern struct {
	static     []string
	vars       []string
	catchAll   bool
	staticSize int // sum(len(static[i]))
}

// Each non-empty element of Pattern.static holds a static segment of
// the path. Each element of vars holds the name of a wildcard variable
// inside the path between two pattern segments. If catchAll is true,
// the last variable is a * pattern that matches any number of trailing
// path elements.
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

// ParsePattern parses the given router pattern
// from the given path.
// A valid pattern always starts with a leading "/".
// Named portions of the path are dynamic path
// segments, of the form :param. They match a segment of the path,
// so they must be preceded by a "/" and followed
// by a "/" or appear at the end of the string.
//
// For example:
//
//	/:name/info
//
// would match /foo/info but not /foo/bar/info.
//
// A catch-all pattern of the form *param may appear
// at the end of the path and matches any number of
// path segments at the end of the pattern.
// It must be preceded by a "/". The value of
// a catch-all parameter will include a leading "/".
//
// For example:
//
//	/foo/*name
//
// would match /foo/info and /foo/bar/info.
func ParsePattern(p string) (*Pattern, error) {
	if httprouter.CleanPath(p) != p {
		return nil, fmt.Errorf("path is not clean")
	}
	var pat Pattern
	if !strings.HasPrefix(p, "/") {
		return nil, fmt.Errorf("path must start with /")
	}
	for len(p) > 0 {
		i := strings.IndexAny(p, ":*")
		if i == -1 {
			pat.static = append(pat.static, p)
			break
		}
		if i == 0 {
			panic("unexpected empty path segment")
		}
		pat.static = append(pat.static, p[0:i])
		if p[i-1] != '/' {
			return nil, fmt.Errorf("no / before wildcard segment")
		}
		p = p[i:]
		i = strings.Index(p, "/")
		if i == -1 {
			pat.static = append(pat.static, "")
			pat.vars = append(pat.vars, p[1:])
			pat.catchAll = p[0] == '*'
			break
		}
		if p[0] == '*' {
			return nil, fmt.Errorf("catch-all route not at end of path")
		}
		v := p[1:i]
		if strings.IndexAny(v, ":*") != -1 {
			return nil, fmt.Errorf("no / before wildcard segment")
		}
		pat.static = append(pat.static, "")
		pat.vars = append(pat.vars, v)
		p = p[i:]
	}
	size := 0
	for _, s := range pat.static {
		size += len(s)
	}
	pat.staticSize = size
	return &pat, nil
}

// Names returns all the parameter names specified
// in the pattern. The caller must not change
// the elements of the returned slice.
func (p *Pattern) Names() []string {
	return p.vars
}

// Path returns a path constructed by interpolating the
// given parameter values. All the parameter values
// must be non-empty. Each value corresponds to
// the parameter at the same position in the slice
// returned by Names.
//
// For example, if the original pattern path
// was /foo/:name/*rest then Names would
// return {"name", "rest"} and Path("a", "/b/c")
// would return /foo/a/b/c.
func (p *Pattern) Path(vals []string) (string, error) {
	if len(vals) != len(p.vars) {
		return "", errgo.Newf("too few parameters")
	}
	size := p.staticSize
	for _, val := range vals {
		size += len(val)
	}
	// TODO allocate exactly the size needed
	path := make([]byte, 0, size)
	for i, elem := range p.static {
		if elem != "" {
			path = append(path, elem...)
			continue
		}
		val := vals[i/2]
		if i == len(p.static)-1 && p.catchAll {
			if !strings.HasPrefix(val, "/") {
				return "", errgo.Newf("catch-all parameter without / prefix")
			}
			val = val[1:]
		} else {
			if val == "" {
				return "", errgo.Newf("empty parameter")
			}
			// TODO check that val contains a / ?
		}
		path = append(path, val...)
	}
	return string(path), nil
}
