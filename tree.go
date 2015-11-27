package hrouter

import (
	"bytes"
	"log"
	"net/http"
	"strings"
)

type Param struct {
	Name  string
	Value string
}

type Params []Param

type Handler interface {
	ServeHTTP(http.ResponseWriter, *http.Request, Params)
}

type node struct {
	// path holds the path segment matched by
	// this node.
	path string

	// firstBytes holds the first byte of the path
	// segment for each child. The path segment
	// of the child does not include the byte held
	// here.
	firstBytes []byte
	child      []*node

	// wild holds any wildcard node that descends from here.
	wild *node

	// catchAll holds any final catchAll node that descends from
	// here. Note that it will always be a leaf if present.
	catchAll *node

	// handler holds the handler registered for this node.
	handler Handler

	// emptyParams holds the handler parameters with names
	// filled out but empty values. This is empty when handler is
	// nil.
	emptyParams []Param
}

// addStaticPrefix adds a route to the given node for the
// given static prefix. The given pattern holds the
// remaining elements of the pattern
// we're adding and all the variable names defined
// by the pattern.
//
// Precondition: pat.static is either empty or its first element is empty.
func (n *node) addStaticPrefix(prefix string, pat pattern, h Handler) {
	common := commonPrefix(prefix, n.path)
	if len(common) < len(n.path) {
		// This node's prefix is too long; split it,
		// ensuring that n.path == common.
		n1 := *n
		childPrefix := n.path[len(common):]
		n1.path = childPrefix[1:]
		*n = node{
			path: common,
		}
		n.addChild(childPrefix[0], &n1)
	}
	// Invariant: common == n.path
	if len(common) < len(prefix) {
		// More to go.
		prefix = prefix[len(common):]
		i := bytes.IndexByte(n.firstBytes, prefix[0])
		if i == -1 {
			// No child found, so make a new one.
			i = n.addChild(prefix[0], &node{
				path: prefix[1:],
			})
		}
		// Descend further into the tree.
		n.child[i].addStaticPrefix(prefix[1:], pat, h)
		return
	}
	// Invariant: common == prefix
	if len(pat.static) == 0 {
		// We've arrived at our destination.
		n.setHandler(h, pat.vars)
		return
	}
	// We're adding a wildcard, which might be a
	// single segment or a final catch-all segment.
	wildPt := &n.wild
	if len(pat.static) == 1 && pat.catchAll {
		wildPt = &n.catchAll
	}
	if *wildPt == nil {
		// No existing wildcard node, so add one.
		*wildPt = new(node)
	}
	n = *wildPt
	pat.static = pat.static[1:]
	// Invariant: pat.static is either empty or its first element is non-empty.
	if len(pat.static) == 0 {
		// We've reach our destination.
		n.setHandler(h, pat.vars)
		return
	}
	// Descend further into the tree
	prefix = pat.static[0]
	pat.static = pat.static[1:]
	n.addStaticPrefix(prefix, pat, h)
}

func (n *node) setHandler(h Handler, vars []string) {
	if n.handler != nil {
		panic("duplicate route")
	}
	n.handler = h
	n.emptyParams = make([]Param, len(vars))
	for i, v := range vars {
		n.emptyParams[i].Name = v
	}
}

func (n *node) addChild(firstByte byte, n1 *node) int {
	n.child = append(n.child, n1)
	n.firstBytes = append(n.firstBytes, firstByte)
	return len(n.child) - 1
}

func (n *node) lookup(path string) (*node, []string) {
	var vars []string
	var catchAll *node
	var catchAllPath string
	for {
		if len(path) < len(n.path) {
			return nil, nil
		}
		var prefix string
		prefix, path = path[0:len(n.path)], path[len(n.path):]
		if prefix != n.path {
			return nil, nil
		}
		if path == "" {
			return n, vars
		}
		if n.catchAll != nil {
			catchAllPath = path
			catchAll = n.catchAll
		}
		i := bytes.IndexByte(n.firstBytes, path[0])
		if i >= 0 {
			path = path[1:]
			n = n.child[i]
			continue
		}
		if n.wild != nil {
			elem, rest := pathElem(path)
			if elem == "" {
				return nil, nil
			}
			vars = append(vars, elem)
			path = rest
			n = n.wild
			continue
		}
		if catchAll != nil {
			vars = append(vars, catchAllPath)
			return catchAll, vars
		}
		return nil, nil
	}
}

func lookup(n *node, path string) (*node, Params) {
	n, vars := n.lookup(path)
	if n == nil {
		return nil, nil
	}
	if n.handler == nil {
		return nil, nil
	}
	if len(vars) == 0 {
		return n, nil
	}
	p := make(Params, len(vars))
	copy(p, n.emptyParams)
	for i, v := range vars {
		p[i].Value = v
	}
	return n, p
}

// commonPrefix returns any prefix that s and t
// have in common.
func commonPrefix(s, t string) string {
	if len(s) > len(t) {
		s, t = t, s
	}
	for i := 0; i < len(s); i++ {
		if s[i] != t[i] {
			return s[0:i]
		}
	}
	return s
}

// pathElem splits the first path element of from p.
func pathElem(p string) (string, string) {
	if i := strings.IndexByte(p, '/'); i >= 0 {
		return p[:i], p[i:]
	}
	return p, ""
}
