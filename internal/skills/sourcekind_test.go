package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/codyconfer/pockit/internal/cache"
	"github.com/codyconfer/pockit/internal/catalog"
	"github.com/codyconfer/pockit/internal/inheritance"
	"github.com/codyconfer/pockit/internal/planning"
)

func TestMixedSourcePoliciesShareBytesWithoutLosingMappings(t *testing.T) {
	store, err := cache.New(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	raw := "https://example.com/tool.sh"
	body := []byte("# shared source\r\n")
	if err := store.Write(planning.ID(raw), cache.Record{URL: raw, Outcome: "success", MIME: "text/plain"}, body); err != nil {
		t.Fatal(err)
	}
	resolved := []inheritance.Resolved{{Key: raw, URL: raw}}
	for role, kinds := range map[string][]string{
		"base":  {"markdown", "gh_file_md", "text", "gh_script", "mcp_endpoint", "slack", "future"},
		"child": {"text", "gh_project"},
	} {
		for _, kind := range kinds {
			resolved[0].Contexts = append(resolved[0].Contexts, inheritance.Context{Role: role, Source: catalog.Source{Kind: kind, URL: raw}})
		}
	}
	plan := planning.Build(resolved, "", store.Status)
	if !plan.Entries[0].Fetchable {
		t.Fatal("mixed source lost fetchability")
	}
	report, err := Build(context.Background(), plan, store, []string{"base", "child"}, Options{})
	if err != nil || report.Failed != 1 || len(report.Results) != 9 {
		t.Fatalf("build: %v %+v", err, report.Results)
	}
	artifacts := map[string][]byte{}
	for _, artifact := range report.Artifacts {
		if _, exists := artifacts[artifact.Path]; exists {
			t.Fatalf("duplicate artifact: %s", artifact.Path)
		}
		artifacts[artifact.Path] = artifact.data
	}
	id := planning.ID(raw)
	for role, want := range map[string][]string{
		"base":  {"documents/" + id + ".txt", "references/" + id + ".md", "scripts/" + id + ".sh"},
		"child": {"documents/" + id + ".txt"},
	} {
		var index []BundleSource
		if err := json.Unmarshal(artifacts[role+"/bundle-index.json"], &index); err != nil {
			t.Fatal(err)
		}
		if len(index) != 1 || len(index[0].LocalPaths) != len(want) {
			t.Fatalf("%s: unexpected index %+v", role, index)
		}
		for i, name := range want {
			if index[0].LocalPaths[i] != name || !bytes.Equal(artifacts[role+"/"+name], body) {
				t.Fatalf("%s: missing or changed content %s", role, name)
			}
		}
	}
	if _, ok := artifacts["base/scripts/README.md"]; !ok {
		t.Fatal("script prerequisites missing")
	}
	if _, ok := artifacts["child/scripts/README.md"]; ok {
		t.Fatal("script artifacts leaked between roles")
	}
	var connections Connections
	if err := json.Unmarshal(artifacts["base/mcp-servers.json"], &connections); err != nil {
		t.Fatal(err)
	}
	if connections.Servers["source-"+id].URL != raw {
		t.Fatal("endpoint missing")
	}
}
