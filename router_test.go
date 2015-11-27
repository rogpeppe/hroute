package hrouter

import (
	"github.com/kr/pretty"
	"log"
	"net/http"
	"reflect"
	"testing"
)

var parsePatternTests = []struct {
	path          string
	expectPattern pattern
	expectError   string
}{{
	path: "/foo/bar",
	expectPattern: pattern{
		static: []string{"/foo/bar"},
	},
}, {
	path: "/foo/:bar",
	expectPattern: pattern{
		static: []string{"/foo/", ""},
		vars:   []string{"bar"},
	},
}, {
	path: "/:x/:y/*end",
	expectPattern: pattern{
		static:   []string{"/", "", "/", "", "/", ""},
		vars:     []string{"x", "y", "end"},
		catchAll: true,
	},
}, {
	path: "/a/b/:x/c/d",
	expectPattern: pattern{
		static: []string{"/a/b/", "", "/c/d"},
		vars:   []string{"x"},
	},
}, {
	path: "/a/b/:x/c/d",
	expectPattern: pattern{
		static: []string{"/a/b/", "", "/c/d"},
		vars:   []string{"x"},
	},
}}

func TestParsePattern(t *testing.T) {
	for i, test := range parsePatternTests {
		t.Logf("test %d: %v", i, test.path)
		pat, err := parsePattern(test.path)
		if test.expectError != "" {
			if err == nil {
				t.Errorf("expected error got nil want %q", test.expectError)
			} else if err.Error() != test.expectError {
				t.Errorf("expected error; got %q want %q", err, test.expectError)
			}
		} else {
			if !reflect.DeepEqual(pat, test.expectPattern) {
				t.Errorf("mismatch; got %#v want %#v", pat, test.expectPattern)
			}
		}
	}
}

type lookupTest struct {
	path           string
	expectParams   Params
	expectNotFound bool
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
		expectParams: Params{{"foo", "arble/something"}},
	}},
}, {
	about: "catch-all route with static",
	add: []string{
		"/*foo",
		"/x/:bar",
	},
	lookups: []lookupTest{{
		path:         "/arble/something",
		expectParams: Params{{"foo", "arble/something"}},
	}, {
		path:         "/x/something",
		expectParams: Params{{"bar", "something"}},
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
}}

func TestLookup(t *testing.T) {
	for i, test := range lookupTests {
		log.Printf("\ntest %d: %v", i, test.about)
		t.Logf("\ntest %d: %v", i, test.about)

		n := &node{
			path: "/",
		}
		for _, p := range test.add {
			pat, err := parsePattern(p)
			if err != nil {
				t.Fatalf("bad path %q: %v", err)
			}
			var prefix string
			prefix, pat.static = pat.static[0], pat.static[1:]
			n.addStaticPrefix(prefix, pat, nopHandler(p))
		}
		pretty.Println(n)
		for _, ltest := range test.lookups {
			log.Printf("- lookup %q", ltest.path)
			t.Logf("- lookup %q", ltest.path)
			resultNode, resultParams := lookup(n, ltest.path)
			if ltest.expectNotFound {
				if resultNode != nil {
					t.Errorf("unexpectedly found result")
				}
				continue
			}
			if resultNode == nil {
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
