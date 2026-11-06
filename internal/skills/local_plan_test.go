package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codyconfer/pockit/internal/cache"
	"github.com/codyconfer/pockit/internal/catalog"
	"github.com/codyconfer/pockit/internal/inheritance"
	"github.com/codyconfer/pockit/internal/planning"
)

func TestResolvedLocalPlanOfflineBundle(t *testing.T) {
	fixtures := []struct{ kind, mime, body string }{
		{"markdown", "text/markdown", "# Original\r\n"},
		{"html", "text/html", "<h1>Original</h1>\r\n"},
		{"text", "text/plain", "original\r\n"},
		{"json", "application/json", "{\"original\":true}\n"},
		{"gh_script", "text/plain", "#!/bin/sh\nexit 1\n"},
		{"mcp_endpoint", "", ""},
		{"gh_project", "", ""},
	}
	resources := []catalog.Source{}
	originals := map[string][]byte{}
	store, err := cache.New(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		raw := "https://example.com/" + fixture.kind
		resources = append(resources, catalog.Source{Kind: fixture.kind, URL: raw})
		if fixture.mime != "" {
			body := []byte(fixture.body)
			originals[raw] = body
			if err := store.Write(planning.ID(raw), cache.Record{URL: raw, Outcome: "success", MIME: fixture.mime}, body); err != nil {
				t.Fatal(err)
			}
		}
	}
	input, err := json.Marshal(map[string]any{"base": map[string]any{"resources": resources}, "child": map[string]any{"resources": []any{}}})
	if err != nil {
		t.Fatal(err)
	}
	c, err := catalog.Parse(input, []byte(`{"type":"object"}`))
	if err != nil {
		t.Fatal(err)
	}
	graph, err := inheritance.Parse([]byte(`{"base":[],"child":["base"]}`), c)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := inheritance.Resolve(c, graph, "child")
	if err != nil {
		t.Fatal(err)
	}
	plan := planning.Build(resolved, "child", store.Status)
	planBytes, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, pi := range []bool{false, true} {
		opts := Options{Out: filepath.Join(t.TempDir(), "bundle"), PiExtension: pi}
		report, err := Generate(context.Background(), plan, store, []string{"child"}, opts)
		if err != nil || report.Failed != 0 {
			t.Fatalf("generate: %v %+v", err, report.Results)
		}
		for _, entry := range plan.Entries {
			for _, mapping := range entry.Mappings {
				if !mapping.Context.Inherited || mapping.Context.Origin != "base" || mapping.Context.Role != "child" {
					t.Fatal("lost selected-role provenance")
				}
				if strings.Contains(mapping.Output, "://") {
					t.Fatal("nonlocal output", mapping.Output)
				}
				if _, err := os.Stat(filepath.Join(opts.Out, mapping.Output)); err != nil {
					t.Fatal("missing mapped output", err)
				}
			}
		}
		body, err := os.ReadFile(filepath.Join(opts.Out, "child", "bundle-index.json"))
		if err != nil {
			t.Fatal(err)
		}
		var index []BundleSource
		if err := json.Unmarshal(body, &index); err != nil {
			t.Fatal(err)
		}
		for _, source := range index {
			original, cached := originals[source.URL]
			if !cached {
				if len(source.LocalPaths) != 0 {
					t.Fatal("uncached pointer/endpoint claims local bytes")
				}
				continue
			}
			if len(source.LocalPaths) != 1 {
				t.Fatalf("missing local content: %+v", source)
			}
			actual, err := os.ReadFile(filepath.Join(opts.Out, "child", source.LocalPaths[0]))
			if err != nil || !bytes.Equal(actual, original) {
				t.Fatal("changed original bytes", source.URL, err)
			}
		}
		opts.Check = true
		checked, err := Generate(context.Background(), plan, store, []string{"child"}, opts)
		if err != nil || checked.NeedsRebuild || checked.Fingerprint != report.Fingerprint {
			t.Fatalf("check: %v %+v", err, checked)
		}
	}
	after, err := json.Marshal(plan)
	if err != nil || !bytes.Equal(after, planBytes) {
		t.Fatal("generation mutated the resolved plan")
	}
}
