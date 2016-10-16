package hroute_test

import (
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/rogpeppe/hroute"
)

var parsePatternTests = []struct {
	path            string
	expectError     string
	expectKeys      []string
	expectPath      string
	expectPathError string
}{{
	path:       "/foo/bar",
	expectPath: "/foo/bar",
}, {
	path:       "/foo/:bar",
	expectKeys: []string{"bar"},
	expectPath: "/foo/0",
}, {
	path:       "/:x/:y/*end",
	expectKeys: []string{"x", "y", "end"},
	expectPath: "/0/1/2",
}, {
	path:       "/a/b/:x/c/d",
	expectKeys: []string{"x"},
	expectPath: "/a/b/0/c/d",
}, {
	path:       "/a/b/:x/c/d",
	expectKeys: []string{"x"},
	expectPath: "/a/b/0/c/d",
}}

func TestParsePattern(t *testing.T) {
	for i, test := range parsePatternTests {
		t.Logf("test %d: %v", i, test.path)
		pat, err := hroute.ParsePattern(test.path)
		if test.expectError != "" {
			if err == nil {
				t.Errorf("expected error got nil want %q", test.expectError)
			} else if err.Error() != test.expectError {
				t.Errorf("expected error; got %q want %q", err, test.expectError)
			}
			continue
		}
		gotKeys := pat.Keys()
		if len(gotKeys) == 0 {
			gotKeys = nil
		}
		if want := test.expectKeys; !reflect.DeepEqual(gotKeys, want) {
			t.Errorf("keys mismatch; got %#v want %#v", gotKeys, want)
			continue
		}
		vals := make([]string, len(pat.Keys()))
		for i := range vals {
			vals[i] = fmt.Sprint(i)
		}
		if pat.CatchAll() {
			vals[len(vals)-1] = "/" + vals[len(vals)-1]
		}
		gotPath, gotPathError := pat.Path(vals...)
		if test.expectPathError != "" {
			if gotPathError == nil || gotPathError.Error() != test.expectPathError {
				t.Errorf("unexpected path error; got %q want %q", gotPathError, test.expectPathError)
			}
		} else {
			if gotPath != test.expectPath {
				t.Errorf("path mismatch; got %q want %q", gotPath, test.expectPath)
			}
		}
	}
}

type lookupTest struct {
	// path holds the path to be looked up.
	// By default, it will be looked up with the GET method
	// but an alternative method can be specified
	// by prefixing the path with the method and a space.
	path string

	// expectHandler holds the handler that's expected
	// to be found when looking up path.
	// If it's nil, pathHandler{methid path} is expected.
	expectHandler hroute.Handler

	expectParams hroute.Params
}

var handlerTests = []struct {
	about   string
	add     []string
	lookups []lookupTest
}{{
	about: "single static route",
	add: []string{
		"/foo",
	},
	lookups: []lookupTest{{
		path: "/foo",
	}},
}, {
	about: "two static routes with shared prefix",
	add: []string{
		"/foobar",
		"/fooey",
	},
	lookups: []lookupTest{{
		path: "/foobar",
	}, {
		path: "/fooey",
	}, {
		path:          "/f",
		expectHandler: hroute.NotFound{},
	}, {
		path:          "/foo",
		expectHandler: hroute.NotFound{},
	}, {
		path:          "/foobaz",
		expectHandler: hroute.NotFound{},
	}, {
		path:          "/foofle",
		expectHandler: hroute.NotFound{},
	}},
}, {
	about: "single wildcard route",
	add: []string{
		"/foo/:bar",
	},
	lookups: []lookupTest{{
		path:          "/foo/something",
		expectHandler: pathHandler{"GET", "/foo/:bar"},
		expectParams:  hroute.Params{{"bar", "something"}},
	}, {
		path: "/foo//",
		expectHandler: hroute.Redirect{
			Path: "/foo/",
			Code: http.StatusMovedPermanently,
		},
	}},
}, {
	about: "two wildcard routes",
	add: []string{
		"/foo/:bar",
		"/arble/:x",
	},
	lookups: []lookupTest{{
		path:          "/foo/something",
		expectHandler: pathHandler{"GET", "/foo/:bar"},
		expectParams:  hroute.Params{{"bar", "something"}},
	}, {
		path:          "/arble/something",
		expectHandler: pathHandler{"GET", "/arble/:x"},
		expectParams:  hroute.Params{{"x", "something"}},
	}},
}, {
	about: "single catch-all route",
	add: []string{
		"/*foo",
	},
	lookups: []lookupTest{{
		path:          "/arble/something",
		expectHandler: pathHandler{"GET", "/*foo"},
		expectParams:  hroute.Params{{"foo", "/arble/something"}},
	}, {
		path:          "/",
		expectHandler: pathHandler{"GET", "/*foo"},
		expectParams:  hroute.Params{{"foo", "/"}},
	}},
}, {
	about: "catch-all route with static",
	add: []string{
		"/*foo",
		"/x/:bar",
	},
	lookups: []lookupTest{{
		path:          "/arble/something",
		expectHandler: pathHandler{"GET", "/*foo"},
		expectParams:  hroute.Params{{"foo", "/arble/something"}},
	}, {
		path:          "/x/something",
		expectHandler: pathHandler{"GET", "/x/:bar"},
		expectParams:  hroute.Params{{"bar", "something"}},
	}},
}, {
	about: "catch-all route with wildcard element at same level",
	add: []string{
		"/*foo",
		"/:bar",
	},
	lookups: []lookupTest{{
		path:          "/arble/something",
		expectHandler: pathHandler{"GET", "/*foo"},
		expectParams:  hroute.Params{{"foo", "/arble/something"}},
	}, {
		path:          "/arble",
		expectHandler: pathHandler{"GET", "/:bar"},
		expectParams:  hroute.Params{{"bar", "arble"}},
	}, {
		path:          "/",
		expectHandler: pathHandler{"GET", "/*foo"},
		expectParams:  hroute.Params{{"foo", "/"}},
	}},
}, {
	about: "path with several wildcards",
	add: []string{
		"/:foo/:bar/:baz",
	},
	lookups: []lookupTest{{
		path:          "/one/two/three",
		expectHandler: pathHandler{"GET", "/:foo/:bar/:baz"},
		expectParams:  hroute.Params{{"foo", "one"}, {"bar", "two"}, {"baz", "three"}},
	}, {
		path:          "/one",
		expectHandler: hroute.NotFound{},
	}, {
		path:          "/one/two",
		expectHandler: hroute.NotFound{},
	}, {
		path:          "/one/two/three/four",
		expectHandler: hroute.NotFound{},
	}},
}, {
	about: "specific path overrides wildcard",
	add: []string{
		"/:foo/bar/baz",
		"/:foo/:x/baz",
	},
	lookups: []lookupTest{{
		path:          "/x/bar/baz",
		expectHandler: pathHandler{"GET", "/:foo/bar/baz"},
		expectParams:  hroute.Params{{"foo", "x"}},
	}, {
		path:          "/y/floof/baz",
		expectHandler: pathHandler{"GET", "/:foo/:x/baz"},
		expectParams:  hroute.Params{{"foo", "y"}, {"x", "floof"}},
	}},
}, {
	about: "no backtracking",
	add: []string{
		"/a/b/c",
		"/a/:x/d",
	},
	lookups: []lookupTest{{
		path:          "/a/b/c",
		expectHandler: pathHandler{"GET", "/a/b/c"},
	}, {
		path:          "/a/xx/d",
		expectHandler: pathHandler{"GET", "/a/:x/d"},
		expectParams:  hroute.Params{{"x", "xx"}},
	}, {
		path:          "/a/b/d",
		expectHandler: hroute.NotFound{},
	}},
}, {
	about: "trailing slash redirect",
	add: []string{
		"/foo/bar/",
		"/foo/baz/blah",
	},
	lookups: []lookupTest{{
		path: "/foo/bar",
		expectHandler: hroute.Redirect{
			Code: 301,
			Path: "/foo/bar/",
		},
	}, {
		path:          "/foo",
		expectHandler: hroute.NotFound{},
	}},
}, {
	about: "trailing slash redirect at node boundary",
	add: []string{
		"/foo/bar/",
		"/foo/barfle",
	},
	lookups: []lookupTest{{
		path: "/foo/bar",
		expectHandler: hroute.Redirect{
			Code: 301,
			Path: "/foo/bar/",
		},
	}, {
		path: "/foo/barfle",
	}},
}, {
	about: "no trailing slash redirect at node boundary",
	add: []string{
		"/foo/bar/arble",
		"/foo/barfle",
	},
	lookups: []lookupTest{{
		path:          "/foo/bar",
		expectHandler: hroute.NotFound{},
	}, {
		path: "/foo/barfle",
	}},
}}

func TestHandlerToUse(t *testing.T) {
	for i, test := range handlerTests {
		log.Printf("\ntest %d: %v", i, test.about)
		t.Logf("\ntest %d: %v", i, test.about)

		r := hroute.New()
		for _, p := range test.add {
			method, path := methodAndPath(p)
			r.Handle(path, pathHandler{method, path}, method)
		}
		for _, ltest := range test.lookups {
			log.Printf("- lookup %q", ltest.path)
			t.Logf("- lookup %q", ltest.path)
			method, path := methodAndPath(ltest.path)
			resultHandler, resultParams := r.HandlerToUse(method, path)
			expectHandler := ltest.expectHandler
			if expectHandler == nil {
				expectHandler = pathHandler{
					method: method,
					path:   path,
				}
			}
			if !reflect.DeepEqual(resultHandler, expectHandler) {
				t.Errorf("unexpected result handler; got %#v want %#v", resultHandler, expectHandler)
			}
			if len(resultParams) == 0 {
				resultParams = nil
			}
			if !reflect.DeepEqual(resultParams, ltest.expectParams) {
				t.Errorf("unexpected result params; got %#v want %#v", resultParams, ltest.expectParams)
			}
		}
	}
}

func methodAndPath(p string) (method, path string) {
	method = "GET"
	path = p
	if i := strings.Index(path, " "); i != -1 {
		method = p[0:i]
		path = path[i+1:]
	}
	return method, path
}

type nopHandler string

func (nopHandler) ServeHTTP(http.ResponseWriter, *http.Request, hroute.Params) {
	panic("nope")
}

type pathHandler struct {
	method string
	path   string
}

func (h pathHandler) ServeHTTP(w http.ResponseWriter, req *http.Request, params hroute.Params) {
}
