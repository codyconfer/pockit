package inheritance

import (
	"encoding/json"
	"testing"

	"github.com/codyconfer/pockit/data"
	"github.com/codyconfer/pockit/internal/catalog"
)

func TestDefaults(t *testing.T) {
	c, e := catalog.Parse(data.Catalog, data.Schema)
	if e != nil {
		t.Fatal(e)
	}
	g, e := Parse(data.Roles, c)
	if e != nil {
		t.Fatal(e)
	}
	for _, n := range c.Names() {
		r, e := Resolve(c, g, n)
		if e != nil {
			t.Fatal(e)
		}
		direct, inherited := 0, 0
		for _, s := range r {
			for _, ctx := range s.Contexts {
				if ctx.Inherited {
					inherited++
				} else {
					direct++
				}
			}
		}
		if direct != len(c[n].Resources) {
			t.Fatalf("%s direct=%d", n, direct)
		}
		// Count each ancestor occurrence, including distinct diamond paths.
		var countAncestors func(string) int
		countAncestors = func(role string) int {
			total := 0
			for _, parent := range g[role] {
				total += len(c[parent].Resources) + countAncestors(parent)
			}
			return total
		}
		if want := countAncestors(n); inherited != want {
			t.Fatalf("%s inherited=%d want=%d", n, inherited, want)
		}
		a, _ := json.Marshal(r)
		again, _ := Resolve(c, g, n)
		b, _ := json.Marshal(again)
		if string(a) != string(b) {
			t.Fatal("nondeterministic resolution")
		}
	}
	if _, e := Resolve(c, g, "absent"); e == nil {
		t.Fatal("accepted unknown selection")
	}
}

func TestInvalidGraphs(t *testing.T) {
	c := catalog.Catalog{"base": {}, "child": {}}
	for _, s := range []string{`{"absent":[]}`, `{"child":["absent"]}`, `{"child":["base","base"]}`, `{"child":["base"],"base":["child"]}`, `{"base":["base"]}`} {
		if _, e := Parse([]byte(s), c); e == nil {
			t.Fatalf("accepted %s", s)
		}
	}
}
func TestSharedURLPreservesAnnotations(t *testing.T) {
	c := catalog.Catalog{"base": {Resources: []catalog.Source{{URL: "https://example.com", Kind: "markdown", Context: "base", Status: "historical"}}}, "child": {Resources: []catalog.Source{{URL: "https://example.com", Kind: "html", Context: "child", Status: "current"}}}}
	g, e := Parse([]byte(`{"child":["base"]}`), c)
	if e != nil {
		t.Fatal(e)
	}
	r, e := Resolve(c, g, "child")
	if e != nil {
		t.Fatal(e)
	}
	if len(r) != 1 || len(r[0].Contexts) != 2 {
		t.Fatalf("lost context: %+v", r)
	}
	if r[0].Contexts[0].Source.Status != "current" || r[0].Contexts[1].Source.Status != "historical" {
		t.Fatal("lost conflicting annotation")
	}
}
