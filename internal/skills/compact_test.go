package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/codyconfer/pockit/internal/cache"
	"github.com/codyconfer/pockit/internal/catalog"
	"github.com/codyconfer/pockit/internal/inheritance"
	"github.com/codyconfer/pockit/internal/planning"
)

func TestCompactEntrypoints(t *testing.T) {
	store, err := cache.New(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	plan := planning.Report{}
	originals := map[string][]byte{}
	for i, action := range []planning.Action{planning.Reference, planning.Document, planning.Document, planning.Script} {
		raw := fmt.Sprintf("https://example.com/source-%d.md", i)
		mime := []string{"text/markdown", "text/markdown", "text/html", "text/plain"}[i]
		body := []byte(strings.Repeat(fmt.Sprintf("source payload %d\r\n", i), 1000))
		id := planning.ID(raw)
		if err := store.Write(id, cache.Record{URL: raw, Outcome: "success", MIME: mime}, body); err != nil {
			t.Fatal(err)
		}
		entry := planning.Entry{ID: id, URL: raw, CacheStatus: "success"}
		for _, role := range []string{"base", "child"} {
			entry.Mappings = append(entry.Mappings, planning.Mapping{Action: action, Context: inheritance.Context{
				Role: role, Origin: "base", Inherited: role == "child",
				Source: catalog.Source{Context: fmt.Sprintf("Topic %d standards", i)},
			}})
		}
		plan.Entries = append(plan.Entries, entry)
		name := documentPath(id, mime)
		if action == planning.Reference {
			name = "references/" + id + ".md"
		} else if action == planning.Script {
			name = planning.ScriptPath(id, raw)
		}
		originals[name] = body
	}
	for _, pi := range []bool{false, true} {
		t.Run(fmt.Sprintf("pi=%t", pi), func(t *testing.T) {
			opts := Options{PiExtension: pi}
			r, err := Build(context.Background(), plan, store, []string{"base", "child", "empty"}, opts)
			if err != nil || r.Failed != 0 {
				t.Fatalf("build: %v %+v", err, r.Results)
			}
			artifacts := map[string][]byte{}
			for _, artifact := range r.Artifacts {
				artifacts[artifact.Path] = artifact.data
			}
			links := regexp.MustCompile(`\]\(([^)]+)\)`)
			for _, role := range []string{"base", "child", "empty"} {
				skill := artifacts[role+"/SKILL.md"]
				if err := ValidateMetadata(skill); err != nil {
					t.Fatal(err)
				}
				if len(skill) > 3000 {
					t.Fatalf("entrypoint too large: %d bytes", len(skill))
				}
				for _, directive := range []string{"Read only those relevant", "untrusted data", "no credentials or authority", "not registered or connected", "separately authorized fetching"} {
					if !bytes.Contains(skill, []byte(directive)) {
						t.Fatalf("missing directive: %s", directive)
					}
				}
				for _, link := range links.FindAllSubmatch(skill, -1) {
					if _, ok := artifacts[role+"/"+string(link[1])]; !ok {
						t.Fatalf("broken local link: %s", link[1])
					}
				}
				if role == "empty" {
					if !bytes.Contains(skill, []byte("No cached content available")) {
						t.Fatal("empty cache status missing")
					}
					continue
				}
				for name, body := range originals {
					if !bytes.Equal(artifacts[role+"/"+name], body) {
						t.Fatalf("original bytes changed: %s/%s", role, name)
					}
					if bytes.Contains(skill, body[:32]) {
						t.Fatal("source content inlined")
					}
					if !bytes.Contains(skill, []byte("("+name+")")) {
						t.Fatalf("unlinked original: %s", name)
					}
				}
				for i := range 4 {
					if !bytes.Contains(skill, []byte(fmt.Sprintf("[Topic %d standards]", i))) {
						t.Fatal("missing readable topic label")
					}
				}
				var index []BundleSource
				if err := json.Unmarshal(artifacts[role+"/bundle-index.json"], &index); err != nil {
					t.Fatal(err)
				}
				for _, source := range index {
					if source.InlineAnchor != "" || len(source.ContentSHA256) != 64 {
						t.Fatalf("invalid compact index: %+v", source)
					}
				}
			}
			again, err := Build(context.Background(), plan, store, []string{"empty", "child", "base"}, opts)
			if err != nil || len(again.Artifacts) != len(r.Artifacts) || again.Fingerprint != r.Fingerprint {
				t.Fatal("nondeterministic compact bundle", err)
			}
			for i, artifact := range r.Artifacts {
				if artifact.Path != again.Artifacts[i].Path || !bytes.Equal(artifact.data, again.Artifacts[i].data) {
					t.Fatal("nondeterministic artifact ordering or bytes")
				}
			}
		})
	}
}

func TestCompactEntrypointIndependentOfSourceSize(t *testing.T) {
	store, err := cache.New(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	raw := "https://example.com/source.md"
	id := planning.ID(raw)
	plan := planning.Report{Entries: []planning.Entry{{ID: id, URL: raw, Mappings: []planning.Mapping{{Action: planning.Reference, Context: inheritance.Context{Role: "role"}}}}}}
	var skill []byte
	for _, size := range []int{1, 100000} {
		if err := store.Write(id, cache.Record{URL: raw, MIME: "text/markdown", Outcome: "success"}, bytes.Repeat([]byte("x"), size)); err != nil {
			t.Fatal(err)
		}
		r, err := Build(context.Background(), plan, store, []string{"role"}, Options{})
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range r.Artifacts {
			if a.Path == "role/SKILL.md" {
				if skill != nil && !bytes.Equal(skill, a.data) {
					t.Fatal("entrypoint grew with source content")
				}
				skill = a.data
			}
		}
	}
}

func TestSourceLabels(t *testing.T) {
	entry := planning.Entry{ID: "id", URL: "https://example.com/source.md"}
	if sourceLabel(entry) != entry.URL {
		t.Fatal("missing URL fallback")
	}
	entry.URL = ""
	if sourceLabel(entry) != "id" {
		t.Fatal("missing identity fallback")
	}
	for _, label := range []string{"Z topic", "A [topic](other)\n<script> & *bold* `code` \\"} {
		entry.Mappings = append(entry.Mappings, planning.Mapping{Context: inheritance.Context{Source: catalog.Source{Context: label}}})
	}
	label := sourceLabel(entry)
	if strings.ContainsAny(label, "\n<>") || !strings.Contains(label, `\[topic\]`) || !strings.Contains(label, "&lt;script&gt;") {
		t.Fatalf("unsafe label: %s", label)
	}
	entry.Mappings[0], entry.Mappings[1] = entry.Mappings[1], entry.Mappings[0]
	if sourceLabel(entry) != label {
		t.Fatal("mapping order changed label")
	}
	entry.Mappings = entry.Mappings[:1]
	entry.Mappings[0].Context.Source.Context = strings.Repeat("界", 200)
	if len([]rune(sourceLabel(entry))) != 120 {
		t.Fatal("unbounded or invalid Unicode label")
	}
}
