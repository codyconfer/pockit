package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codyconfer/pockit/internal/cache"
	"github.com/codyconfer/pockit/internal/inheritance"
	"github.com/codyconfer/pockit/internal/planning"
)

func TestOfflineBundleDocumentsAndLinkContext(t *testing.T) {
	store, err := cache.New(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	roles := []string{"base", "child"}
	type fixture struct {
		url, mime string
		action    planning.Action
		body      []byte
		cached    bool
	}
	fixtures := []fixture{
		{"https://github.com/example/repo/blob/main/docs/guide.md", "text/markdown", planning.Reference, []byte("# Guide\n[Next](next.md#section)\n[Missing](../missing.md)\n"), true},
		{"https://github.com/example/repo/blob/main/docs/next.md", "text/markdown", planning.Reference, []byte("# Next\n"), true},
		{"https://example.com/review/", "text/html; charset=utf-8", planning.Document, []byte("<h1>Review</h1>\r\n"), true},
		{"https://example.com/rules.json", "application/json", planning.Document, []byte("{\"rule\":true}\n"), true},
		{"https://example.com/raw", "application/octet-stream", planning.Document, []byte{0xff, 0, 0x80}, true},
		{"https://example.com/empty", "text/plain", planning.Document, []byte{}, true},
		{"https://example.com/unavailable", "text/html", planning.Document, nil, false},
		{"https://example.com/repository", "", planning.Pointer, nil, false},
	}
	plan := planning.Report{}
	for _, f := range fixtures {
		id := planning.ID(f.url)
		if f.cached {
			if err := store.Write(id, cache.Record{URL: f.url, MIME: f.mime, Outcome: "success"}, f.body); err != nil {
				t.Fatal(err)
			}
		}
		entry := planning.Entry{ID: id, URL: f.url, CacheStatus: store.Status(id)}
		for _, role := range roles {
			entry.Mappings = append(entry.Mappings, planning.Mapping{Action: f.action, Context: inheritance.Context{Role: role}})
		}
		plan.Entries = append(plan.Entries, entry)
	}
	r, err := Build(context.Background(), plan, store, roles, Options{})
	if err != nil || r.Failed != 2 {
		t.Fatalf("build: %v, failures %d", err, r.Failed)
	}
	artifacts := map[string][]byte{}
	for _, a := range r.Artifacts {
		artifacts[a.Path] = a.data
	}
	for _, role := range roles {
		var index []BundleSource
		if err := json.Unmarshal(artifacts[role+"/bundle-index.json"], &index); err != nil {
			t.Fatal(err)
		}
		if len(index) != len(fixtures) {
			t.Fatalf("incomplete index for %s", role)
		}
		byURL := map[string]BundleSource{}
		for i, source := range index {
			if i > 0 && index[i-1].ID >= source.ID {
				t.Fatal("unsorted or duplicate index")
			}
			byURL[source.URL] = source
		}
		for _, f := range fixtures {
			source := byURL[f.url]
			if !f.cached {
				if len(source.LocalPaths) != 0 {
					t.Fatal("unavailable source claims local content")
				}
				continue
			}
			if len(source.LocalPaths) != 1 || source.MIME != f.mime {
				t.Fatalf("bad index entry: %+v", source)
			}
			path := source.LocalPaths[0]
			body, ok := artifacts[role+"/"+path]
			if !ok || !bytes.Equal(body, f.body) {
				t.Fatalf("original bytes missing: %s", path)
			}
			if !strings.Contains(string(artifacts[role+"/SKILL.md"]), "("+path+")") {
				t.Fatalf("unlinked artifact: %s", path)
			}
		}
		base, _ := url.Parse(byURL[fixtures[0].url].RelativeLinkBaseURL)
		link, _ := url.Parse("next.md#section")
		resolved := base.ResolveReference(link)
		resolved.Fragment = ""
		if len(byURL[resolved.String()].LocalPaths) != 1 {
			t.Fatal("relative link cannot find packaged sibling")
		}
		anchor, _ := url.Parse("#local")
		if base.ResolveReference(anchor).String() != fixtures[0].url+"#local" {
			t.Fatal("fragment link lost source filename")
		}
	}
	again, err := Build(context.Background(), plan, store, []string{"child", "base"}, Options{})
	if err != nil || len(again.Artifacts) != len(r.Artifacts) {
		t.Fatal("repeat build failed", err)
	}
	for i, a := range r.Artifacts {
		if a.Path != again.Artifacts[i].Path || !bytes.Equal(a.data, again.Artifacts[i].data) {
			t.Fatal("nondeterministic bundle")
		}
	}
}

func TestDocumentExtensionsAreFixed(t *testing.T) {
	for _, tt := range []struct{ mime, ext string }{
		{"text/markdown", ".md"}, {"text/plain; charset=utf-8", ".txt"},
		{"text/html", ".html"}, {"application/json", ".json"},
		{"../../escape", ".bin"}, {"", ".bin"},
	} {
		if got := documentPath("id", tt.mime); got != "documents/id"+tt.ext {
			t.Fatalf("unsafe or wrong extension: %s", got)
		}
	}
}
