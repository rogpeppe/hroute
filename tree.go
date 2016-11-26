package hroute

import (
	"bytes"
	"net/http"
	"strings"
)

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

	// handlers holds the handlers registered for this node,
	// indexed by method name.
	handlers map[string]handlerEntry
}

type handlerEntry struct {
	// handler holds the handler registered for a method in a node.
	handler RouteHandler

	// emptyParams holds the handler parameters with names
	// filled out but empty values.
	emptyParams Params

	// pattern holds the pattern that was used to register the entry.
	pattern *Pattern
}

func (n *node) addRoute(pat *Pattern, method string, h Handler) {
	var prefix string
	pat1 := *pat
	prefix, pat1.static = pat1.static[0], pat1.static[1:]
	n.addStaticPrefix(prefix, &pat1, method, h)
}

// addStaticPrefix adds a route to the given node for the given static
// prefix. The given pattern holds the remaining elements of the pattern
// we're adding and all the variable names defined by the pattern.
//
// Precondition: pat.static is either empty or its first element is empty.
func (n *node) addStaticPrefix(prefix string, pat *Pattern, method string, h Handler) {
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
		n.child[i].addStaticPrefix(prefix[1:], pat, method, h)
		return
	}
	// Invariant: common == prefix
	if len(pat.static) == 0 {
		// We've arrived at our destination.
		n.setHandler(method, h, pat)
		return
	}
	// We're adding a wildcard, which might be a single segment or a
	// final catch-all segment.
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
		// We've reached our destination.
		n.setHandler(method, h, pat)
		return
	}
	// Descend further into the tree
	prefix = pat.static[0]
	pat.static = pat.static[1:]
	n.addStaticPrefix(prefix, pat, method, h)
}

func (n *node) setHandler(method string, h Handler, pat *Pattern) {
	if n.handlers == nil {
		n.handlers = make(map[string]handlerEntry)
	}
	if n.handlers[method].handler != nil {
		panic("duplicate route")
	}

	emptyParams := make([]Param, len(vars))
	for i, v := range vars {
		emptyParams[i].Key = v
	}
	n.handlers[method] = handlerEntry{
		handler:     h,
		emptyParams: emptyParams,
		pattern: pat,
	}
}

func (n *node) addChild(firstByte byte, n1 *node) int {
	n.child = append(n.child, n1)
	n.firstBytes = append(n.firstBytes, firstByte)
	return len(n.child) - 1
}

func (n *node) lookup(path string) (*node, []string) {
	origPath := path
	var vars []string
	var catchAll *node
	var catchAllPath string
	var catchAllVars []string
	for {
		if len(path) < len(n.path) {
			break
		}
		var prefix string
		prefix, path = path[0:len(n.path)], path[len(n.path):]
		if prefix != n.path {
			break
		}
		if path == "" {
			return n, vars
		}
		if n.catchAll != nil {
			catchAllPath = path
			catchAll = n.catchAll
			catchAllVars = vars
		}
		i := bytes.IndexByte(n.firstBytes, path[0])
		if i >= 0 {
			path = path[1:]
			n = n.child[i]
			continue
		}
		if n.wild == nil {
			break
		}
		elem, rest := pathElem(path)
		if elem == "" {
			break
		}
		vars = append(vars, elem)
		path = rest
		n = n.wild
	}
	if catchAll != nil {
		// The catchAll path needs to include the / that precedes it.
		// We're guaranteed that there *is* a preceding / because
		// the pattern parsing ensures it.
		vars = append(catchAllVars, origPath[len(origPath)-len(catchAllPath)-1:])
		return catchAll, vars
	}
	return nil, nil
}

// getValue looks up the given path and method and
// returns any handler found along with the parameters
// to be passed to that handler.
// It also returns any node found for the path, even if no handler
// was found.
func (n *node) getValue(method, path string) (h RouteHandler, p Params, foundNode *node) {
	foundNode, vars := n.lookup(path)
	if foundNode == nil {
		return nil, nil, nil
	}
	entry := foundNode.handlers[method]
	if entry.handler == nil {
		// No handler found directly in this node, but if
		// there's a catchAll handler, we can fall back to that.
		if foundNode.catchAll == nil {
			return nil, nil, foundNode
		}
		entry = foundNode.catchAll.handlers[method]
		if entry.handler == nil {
			return nil, nil, foundNode
		}
		vars = append(vars, "/")
	}
	if len(vars) == 0 {
		return entry.handler, nil, foundNode
	}
	p = make(Params, len(vars))
	copy(p, entry.emptyParams)
	for i, v := range vars {
		p[i].Value = v
	}
	return entry.handler, p, foundNode
}

func (n *node) findCaseInsensitivePath(path string, redir bool) (string, bool) {
	// TODO
	return "", false
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
