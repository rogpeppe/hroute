package hrouter

import (
	//	"github.com/kr/pretty"
	"log"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

var parsePatternTests = []struct {
	path          string
	expectPattern Pattern
	expectError   string
}{{
	path: "/foo/bar",
	expectPattern: Pattern{
		static: []string{"/foo/bar"},
	},
}, {
	path: "/foo/:bar",
	expectPattern: Pattern{
		static: []string{"/foo/", ""},
		vars:   []string{"bar"},
	},
}, {
	path: "/:x/:y/*end",
	expectPattern: Pattern{
		static:   []string{"/", "", "/", "", "/", ""},
		vars:     []string{"x", "y", "end"},
		catchAll: true,
	},
}, {
	path: "/a/b/:x/c/d",
	expectPattern: Pattern{
		static: []string{"/a/b/", "", "/c/d"},
		vars:   []string{"x"},
	},
}, {
	path: "/a/b/:x/c/d",
	expectPattern: Pattern{
		static: []string{"/a/b/", "", "/c/d"},
		vars:   []string{"x"},
	},
}}

func TestParsePattern(t *testing.T) {
	for i, test := range parsePatternTests {
		t.Logf("test %d: %v", i, test.path)
		pat, err := ParsePattern(test.path)
		if test.expectError != "" {
			if err == nil {
				t.Errorf("expected error got nil want %q", test.expectError)
			} else if err.Error() != test.expectError {
				t.Errorf("expected error; got %q want %q", err, test.expectError)
			}
		} else {
			test.expectPattern.staticSize = len(strings.Join(test.expectPattern.static, ""))
			if !reflect.DeepEqual(pat, &test.expectPattern) {
				t.Errorf("mismatch; got %#v want %#v", pat, test.expectPattern)
			}
		}
	}
}

type lookupTest struct {
	path           string
	expectParams   Params
	expectNotFound bool
	expectTSR      string
}

var lookupTests = []struct {
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
		path:           "/f",
		expectNotFound: true,
	}, {
		path:           "/foo",
		expectNotFound: true,
	}, {
		path:           "/foobaz",
		expectNotFound: true,
	}, {
		path:           "/foofle",
		expectNotFound: true,
	}},
}, {
	about: "single wildcard route",
	add: []string{
		"/foo/:bar",
	},
	lookups: []lookupTest{{
		path:         "/foo/something",
		expectParams: Params{{"bar", "something"}},
	}, {
		path:           "/foo//",
		expectNotFound: true,
	}},
}, {
	about: "two wildcard routes",
	add: []string{
		"/foo/:bar",
		"/arble/:x",
	},
	lookups: []lookupTest{{
		path:         "/foo/something",
		expectParams: Params{{"bar", "something"}},
	}, {
		path:         "/arble/something",
		expectParams: Params{{"x", "something"}},
	}},
}, {
	about: "single catch-all route",
	add: []string{
		"/*foo",
	},
	lookups: []lookupTest{{
		path:         "/arble/something",
		expectParams: Params{{"foo", "/arble/something"}},
	}, {
		path:         "/",
		expectParams: Params{{"foo", "/"}},
	}},
}, {
	about: "catch-all route with static",
	add: []string{
		"/*foo",
		"/x/:bar",
	},
	lookups: []lookupTest{{
		path:         "/arble/something",
		expectParams: Params{{"foo", "/arble/something"}},
	}, {
		path:         "/x/something",
		expectParams: Params{{"bar", "something"}},
	}},
}, {
	about: "catch-all route with wildcard element at same level",
	add: []string{
		"/*foo",
		"/:bar",
	},
	lookups: []lookupTest{{
		path:         "/arble/something",
		expectParams: Params{{"foo", "/arble/something"}},
	}, {
		path:         "/arble",
		expectParams: Params{{"bar", "arble"}},
	}, {
		path:         "/",
		expectParams: Params{{"foo", "/"}},
	}},
}, {
	about: "path with several wildcards",
	add: []string{
		"/:foo/:bar/:baz",
	},
	lookups: []lookupTest{{
		path:         "/one/two/three",
		expectParams: Params{{"foo", "one"}, {"bar", "two"}, {"baz", "three"}},
	}, {
		path:           "/one",
		expectNotFound: true,
	}, {
		path:           "/one/two",
		expectNotFound: true,
	}, {
		path:           "/one/two/three/four",
		expectNotFound: true,
	}},
}, {
	about: "specific path overrides wildcard",
	add: []string{
		"/:foo/bar/baz",
		"/:foo/:x/baz",
	},
	lookups: []lookupTest{{
		path:         "/x/bar/baz",
		expectParams: Params{{"foo", "x"}},
	}, {
		path:         "/y/floof/baz",
		expectParams: Params{{"foo", "y"}, {"x", "floof"}},
	}},
}, {
	about: "no backtracking",
	add: []string{
		"/a/b/c",
		"/a/:x/d",
	},
	lookups: []lookupTest{{
		path: "/a/b/c",
	}, {
		path:         "/a/xx/d",
		expectParams: Params{{"x", "xx"}},
	}, {
		path:           "/a/b/d",
		expectNotFound: true,
	}},
}, {
	about: "trailing slash redirect",
	add: []string{
		"/foo/bar/",
		"/foo/baz/blah",
	},
	lookups: []lookupTest{{
		path:           "/foo/bar",
		expectTSR:      "/foo/bar/",
		expectNotFound: true,
	}, {
		path:           "/foo",
		expectNotFound: true,
	}},
}, {
	about: "trailing slash redirect at node boundary",
	add: []string{
		"/foo/bar/",
		"/foo/barfle",
	},
	lookups: []lookupTest{{
		path:           "/foo/bar",
		expectTSR:      "/foo/bar/",
		expectNotFound: true,
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
		path:           "/foo/bar",
		expectNotFound: true,
	}, {
		path: "/foo/barfle",
	}},
}}

func TestLookup(t *testing.T) {
	for i, test := range lookupTests {
		log.Printf("\ntest %d: %v", i, test.about)
		t.Logf("\ntest %d: %v", i, test.about)

		n := &node{
			path: "/",
		}
		for _, p := range test.add {
			pat, err := ParsePattern(p)
			if err != nil {
				t.Fatalf("cannot parse pattern %q: %v", err)
			}
			n.addRoute(pat, "GET", nopHandler(p))
		}
		//pretty.Println(n)
		for _, ltest := range test.lookups {
			log.Printf("- lookup %q", ltest.path)
			t.Logf("- lookup %q", ltest.path)
			resultHandler, resultParams, resultNode := n.getValue("GET", ltest.path)
			var resultTSR string
			if resultHandler == nil && (resultNode == nil || len(resultNode.handlers) == 0) {
				resultTSR = n.slashRedirect(ltest.path)
			}
			if resultTSR != ltest.expectTSR {
				t.Errorf("unexpected trailing-slash-redirect value; got %v want %v", resultTSR, ltest.expectTSR)
			}
			if ltest.expectNotFound {
				if resultHandler != nil {
					t.Errorf("unexpectedly found result %q", resultHandler)
				}
				continue
			}
			if resultHandler == nil {
				t.Errorf("expected found but it wasn't")
				continue
			}
			if !reflect.DeepEqual(resultParams, ltest.expectParams) {
				t.Errorf("unexpected result params; got %#v want %#v", resultParams, ltest.expectParams)
			}
		}
	}
}

type nopHandler string

func (nopHandler) ServeHTTP(http.ResponseWriter, *http.Request, Params) {
	panic("nope")
}
