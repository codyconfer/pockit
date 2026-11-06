package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/codyconfer/pockit/internal/cache"
	"github.com/codyconfer/pockit/internal/inheritance"
	"github.com/codyconfer/pockit/internal/planning"
)

func TestLinkedMarkdownAndOfflineUpdates(t *testing.T) {
	ctx := context.Background()
	store, err := cache.New(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	plan := planning.Report{}
	bodies := [][]byte{[]byte("# Reference\r\n\r\n[Other](other.md)\r\n"), []byte("---\nname: original\ndescription: source\n---\n\n# Document\n"), {}}
	for i, action := range []planning.Action{planning.Reference, planning.Document, planning.Reference} {
		url := "https://example.com/" + []string{"ref.md", "other.md", "empty.md"}[i]
		id := planning.ID(url)
		if err := store.Write(id, cache.Record{URL: url, Outcome: "success", MIME: "text/markdown; charset=utf-8"}, bodies[i]); err != nil {
			t.Fatal(err)
		}
		plan.Entries = append(plan.Entries, planning.Entry{ID: id, URL: url, CacheStatus: "success", Mappings: []planning.Mapping{{Action: action, Context: inheritance.Context{Role: "reviewer"}}}})
	}
	out := filepath.Join(t.TempDir(), "bundle")
	opts := Options{Out: out, Check: true}
	missing, err := Generate(ctx, plan, store, []string{"reviewer"}, opts)
	if err != nil || !missing.NeedsRebuild || missing.Written != 0 {
		t.Fatalf("missing check: %v %+v", err, missing.Changed)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("check wrote output")
	}
	opts.Check = false
	built, err := Generate(ctx, plan, store, []string{"reviewer"}, opts)
	if err != nil {
		t.Fatal(err)
	}
	skill, err := os.ReadFile(filepath.Join(out, "reviewer", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range bodies {
		if len(body) > 0 && bytes.Contains(skill, body) {
			t.Fatal("source body inlined into entrypoint")
		}
	}
	var sources []BundleSource
	index, _ := os.ReadFile(filepath.Join(out, "reviewer", "bundle-index.json"))
	if err := json.Unmarshal(index, &sources); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(index, []byte(`"inline_anchor"`)) {
		t.Fatal("index advertises nonexistent inline anchors")
	}
	for _, source := range sources {
		if source.InlineAnchor != "" || len(source.ContentSHA256) != 64 || len(source.LocalPaths) != 1 {
			t.Fatalf("invalid identity or paths: %+v", source)
		}
		for i, entry := range plan.Entries {
			if entry.ID == source.ID {
				body, err := os.ReadFile(filepath.Join(out, "reviewer", source.LocalPaths[0]))
				if err != nil || !bytes.Equal(body, bodies[i]) {
					t.Fatal("original bytes missing", err)
				}
			}
		}
	}
	opts.Check = true
	clean, err := Generate(ctx, plan, store, []string{"reviewer"}, opts)
	if err != nil || clean.NeedsRebuild || clean.Fingerprint != built.Fingerprint {
		t.Fatal("unchanged bundle not clean", err)
	}
	entry := plan.Entries[0]
	if err := store.Write(entry.ID, cache.Record{URL: entry.URL, Outcome: "success", MIME: "text/markdown"}, []byte("# Changed\n")); err != nil {
		t.Fatal(err)
	}
	changed, err := Generate(ctx, plan, store, []string{"reviewer"}, opts)
	if err != nil || !changed.NeedsRebuild || changed.Fingerprint == built.Fingerprint {
		t.Fatal("content update missed", err)
	}
	opts.Check, opts.Overwrite = false, true
	rebuilt, err := Generate(ctx, plan, store, []string{"reviewer"}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if rebuilt.Fingerprint != changed.Fingerprint {
		t.Fatal("nondeterministic fingerprint")
	}
	updatedSkill, err := os.ReadFile(filepath.Join(out, "reviewer", "SKILL.md"))
	if err != nil || !bytes.Equal(skill, updatedSkill) {
		t.Fatal("source content update changed compact entrypoint", err)
	}
	opts.Check, opts.Overwrite = true, false
	path := filepath.Join(out, "reviewer", "SKILL.md")
	if err := os.WriteFile(path, []byte("edited"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(out, "reviewer", "sources.json")); err != nil {
		t.Fatal(err)
	}
	dirty, err := Generate(ctx, plan, store, []string{"reviewer"}, opts)
	if err != nil || !dirty.NeedsRebuild || len(dirty.Changed) != 2 {
		t.Fatalf("disk changes missed: %v %v", err, dirty.Changed)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "target"), path); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(ctx, plan, store, []string{"reviewer"}, opts); !errors.Is(err, ErrRefused) {
		t.Fatal("symlink check not refused", err)
	}
}
