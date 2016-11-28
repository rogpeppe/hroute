package hroute

import (
	"bytes"
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

	// handlers holds the handlers registered for this node.
	// There is at most one entry for a given method.
	handlers []handlerEntry
}

type handlerEntry struct {
	// method holds the method the entry is registered for.
	// If this is "*", the entry serves all methods that
	// aren't registered specifically.
	method string

	// handler holds the handler registered for a method in a node.
	handler RouteHandler

	// pattern holds the pattern that was used to register the entry.
	pattern *Pattern
}

func (n *node) addRoute(pat *Pattern, method string, h RouteHandler) {
	var prefix string
	pat1 := *pat
	prefix, pat1.static = pat1.static[0], pat1.static[1:]
	n.addStaticPrefix(prefix, &pat1, method, h, pat)
}

func (n *node) entryForMethod(method string) *handlerEntry {
	for i := range n.handlers {
		e := &n.handlers[i]
		if e.method == "*" || e.method == method {
			return e
		}
	}
	return nil
}

// addStaticPrefix adds a route to the given node for the given static
// prefix. The given pattern holds the remaining elements of the pattern
// we're adding and all the variable names defined by the pattern.
//
// Precondition: pat.static is either empty or its first element is empty.
func (n *node) addStaticPrefix(prefix string, pat *Pattern, method string, h RouteHandler, origPat *Pattern) {
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
		n.child[i].addStaticPrefix(prefix[1:], pat, method, h, origPat)
		return
	}
	// Invariant: common == prefix
	if len(pat.static) == 0 {
		// We've arrived at our destination.
		n.setHandler(method, h, origPat)
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
		n.setHandler(method, h, origPat)
		return
	}
	// Descend further into the tree
	prefix = pat.static[0]
	pat.static = pat.static[1:]
	n.addStaticPrefix(prefix, pat, method, h, origPat)
}

func (n *node) setHandler(method string, h RouteHandler, pat *Pattern) {
	oldEntry := n.entryForMethod(method)
	if oldEntry != nil && oldEntry.method == method {
		panic("duplicate route")
	}
	n.handlers = append(n.handlers, handlerEntry{
		method:  method,
		handler: h,
		pattern: pat,
	})
	if oldEntry == nil {
		return
	}
	// There was an old matching entry which must be a
	// wildcard method at the end of the slice, so keep it
	// at the end by swapping it with the entry we've just
	// added. This means we can continue to do a simple
	// linear search in entryForMethod and have it pick up
	// the non-wildcard-method handlers first.
	hlen := len(n.handlers)
	n.handlers[hlen-2], n.handlers[hlen-1] = n.handlers[hlen-1], n.handlers[hlen-2]
}

func (n *node) addChild(firstByte byte, n1 *node) int {
	n.child = append(n.child, n1)
	n.firstBytes = append(n.firstBytes, firstByte)
	return len(n.child) - 1
}

func (n *node) lookup(path string, maxParams int) (*node, Params) {
	origPath := path
	var params Params
	var catchAll *node
	var catchAllPath string
	var catchAllParams Params
lookupLoop:
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
			return n, params
		}
		if n.catchAll != nil {
			catchAllPath = path
			catchAll = n.catchAll
			catchAllParams = params
		}
		first := path[0]
		for i, b := range n.firstBytes {
			if first == b {
				path = path[1:]
				n = n.child[i]
				continue lookupLoop
			}
		}
		if n.wild == nil {
			break
		}
		elem, rest := pathElem(path)
		if elem == "" {
			break
		}
		if params == nil {
			params = make(Params, 0, maxParams)
		}
		params = append(params, Param{
			Value: elem,
		})
		path = rest
		n = n.wild
	}
	if catchAll != nil {
		// The catchAll path needs to include the / that precedes it.
		// We're guaranteed that there *is* a preceding / because
		// the pattern parsing ensures it.
		params = append(catchAllParams, Param{
			Value: origPath[len(origPath)-len(catchAllPath)-1:],
		})
		return catchAll, params
	}
	return nil, nil
}

// getValue looks up the given path and method and
// returns any handler found along with the parameters
// to be passed to that handler.
// It also returns any node found for the path, even if no handler
// was found.
func (n *node) getValue(method, path string, maxParams int) (h RouteHandler, p Params, pat *Pattern, foundNode *node) {
	foundNode, params := n.lookup(path, maxParams)
	if foundNode == nil {
		return nil, nil, nil, nil
	}
	entry := foundNode.entryForMethod(method)
	if entry == nil {
		// No handler found directly in this node, but if
		// there's a catchAll handler, we can fall back to that.
		if foundNode.catchAll == nil {
			// No catchAll handler to fall back to.
			return nil, nil, nil, foundNode
		}
		entry = foundNode.catchAll.entryForMethod(method)
		if entry == nil {
			return nil, nil, nil, foundNode
		}
		params = append(params, Param{
			Value: "/",
		})
	}
	if len(params) == 0 {
		return entry.handler, nil, entry.pattern, foundNode
	}
	// Fill in the keys that were used to register this particular
	// handler.
	for i, key := range entry.pattern.Keys() {
		params[i].Key = key
	}
	return entry.handler, params, entry.pattern, foundNode
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
