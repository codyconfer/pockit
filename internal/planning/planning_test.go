package planning

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/codyconfer/pockit/data"
	"github.com/codyconfer/pockit/internal/catalog"
	"github.com/codyconfer/pockit/internal/inheritance"
)

func TestEveryContextAccountedFor(t *testing.T) {
	c, e := catalog.Parse(data.Catalog, data.Schema)
	if e != nil {
		t.Fatal(e)
	}
	g, e := inheritance.Parse(data.Roles, c)
	if e != nil {
		t.Fatal(e)
	}
	r, e := inheritance.Resolve(c, g, "")
	if e != nil {
		t.Fatal(e)
	}
	plan := Build(r, "", nil)
	// Derive the expected count from the input graph, not a fixed base role.
	var count func(string) int
	count = func(role string) int {
		total := len(c[role].Resources)
		for _, parent := range g[role] {
			total += count(parent)
		}
		return total
	}
	want, got := 0, 0
	for _, n := range c.Names() {
		want += count(n)
	}
	for _, entry := range plan.Entries {
		got += len(entry.Mappings)
		for _, m := range entry.Mappings {
			if m.Output == "" || m.Action == "" {
				t.Fatal("unaccounted mapping")
			}
		}
	}
	if got != want {
		t.Fatalf("contexts=%d want=%d", got, want)
	}
	a, _ := json.Marshal(plan)
	b, _ := json.Marshal(Build(r, "", nil))
	if string(a) != string(b) {
		t.Fatal("nondeterministic plan")
	}
}
func TestLocalBundleMappings(t *testing.T) {
	for _, test := range []struct {
		kind      string
		action    Action
		output    string
		fetchable bool
	}{
		{"markdown", Reference, "role/references/ID.md", true},
		{"gh_script", Script, "role/scripts/ID.sh", true},
		{"html", Document, "role/bundle-index.json", true},
		{"text", Document, "role/bundle-index.json", true},
		{"json", Document, "role/bundle-index.json", true},
		{"mcp_endpoint", Endpoint, "role/mcp-servers.json", false},
		{"gh_project", Pointer, "role/sources.json", false},
		{"future", Unsupported, "role/sources.json", false},
	} {
		t.Run(test.kind, func(t *testing.T) {
			resolved := []inheritance.Resolved{{Key: "source", URL: "https://example.com/source.sh", Contexts: []inheritance.Context{{Role: "role", Origin: "base", Inherited: true, Source: catalog.Source{Kind: test.kind}}}}}
			calls := 0
			plan := Build(resolved, "role", func(id string) string {
				calls++
				if id != ID("source") {
					t.Fatal("changed identity")
				}
				return "success"
			})
			entry := plan.Entries[0]
			mapping := entry.Mappings[0]
			want := strings.ReplaceAll(test.output, "ID", entry.ID)
			if calls != 1 || entry.CacheStatus != "success" || entry.Fetchable != test.fetchable || mapping.Output != want || mapping.Action != test.action {
				t.Fatalf("mapping: %+v entry: %+v calls=%d", mapping, entry, calls)
			}
			if !mapping.Context.Inherited || mapping.Context.Origin != "base" || plan.Summary[string(test.action)] != 1 {
				t.Fatal("lost provenance or summary")
			}
		})
	}
}

func TestDispositions(t *testing.T) {
	for kind, want := range map[string]Action{"markdown": Reference, "gh_file_md": Reference, "html": Document, "text": Document, "json": Document, "slack": Pointer, "mcp_grafana": Endpoint, "future": Unsupported, "gh_script": Script} {
		if got := Classify(kind, "https://example.com"); got != want {
			t.Errorf("%s=%s", kind, got)
		}
	}
	if Classify("markdown", "") != Missing {
		t.Fatal("missing")
	}
}
