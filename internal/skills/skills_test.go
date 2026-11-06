package skills

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/codyconfer/pockit/data"
	"github.com/codyconfer/pockit/internal/cache"
	"github.com/codyconfer/pockit/internal/catalog"
	"github.com/codyconfer/pockit/internal/inheritance"
	"github.com/codyconfer/pockit/internal/planning"
)

func TestDefaultSelfContainedDeterministicSkills(t *testing.T) {
	c, e := catalog.Parse(data.Catalog, data.Schema)
	if e != nil {
		t.Fatal(e)
	}
	g, e := inheritance.Parse(data.Roles, c)
	if e != nil {
		t.Fatal(e)
	}
	resolved, e := inheritance.Resolve(c, g, "")
	if e != nil {
		t.Fatal(e)
	}
	store, e := cache.New(filepath.Join(t.TempDir(), "cache"))
	if e != nil {
		t.Fatal(e)
	}
	plan := planning.Build(resolved, "", store.Status)
	content := []byte("---\nname: original-source\ndescription: Test source content\n---\n\n# Original source\n\nDo not alter these bytes.\n")
	for _, entry := range plan.Entries {
		if entry.Fetchable {
			if e := store.Write(entry.ID, cache.Record{URL: entry.URL, Outcome: "success", MIME: "text/markdown"}, content); e != nil {
				t.Fatal(e)
			}
		}
	}
	opts := Options{Out: filepath.Join(t.TempDir(), "output"), DryRun: true}
	r, e := Generate(context.Background(), plan, store, c.Names(), opts)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(opts.Out); !os.IsNotExist(e) {
		t.Fatal("dry run wrote files")
	}
	opts.DryRun = false
	r, e = Generate(context.Background(), plan, store, c.Names(), opts)
	if e != nil {
		t.Fatal(e)
	}
	wrappers := 0
	for _, a := range r.Artifacts {
		if filepath.Base(a.Path) == "SKILL.md" {
			wrappers++
		}
		b, e := os.ReadFile(filepath.Join(opts.Out, a.Path))
		if e != nil || string(b) != string(a.data) {
			t.Fatalf("artifact mismatch %s", a.Path)
		}
	}
	if wrappers != len(c) || r.Failed != 0 {
		t.Fatalf("wrappers=%d want=%d failures=%d", wrappers, len(c), r.Failed)
	}
	if _, e = Generate(context.Background(), plan, store, c.Names(), opts); e == nil {
		t.Fatal("overwrite not refused")
	}
	opts.Overwrite = true
	again, e := Generate(context.Background(), plan, store, c.Names(), opts)
	if e != nil {
		t.Fatal(e)
	}
	if len(again.Artifacts) != len(r.Artifacts) {
		t.Fatal("artifact count changed")
	}
	for i, a := range r.Artifacts {
		if a.Path != again.Artifacts[i].Path || string(a.data) != string(again.Artifacts[i].data) {
			t.Fatal("nondeterministic generation")
		}
	}
	for _, entry := range plan.Entries {
		for _, mapping := range entry.Mappings {
			if mapping.Action != planning.Reference {
				continue
			}
			body, err := os.ReadFile(filepath.Join(opts.Out, mapping.Output))
			if err != nil || string(body) != string(content) {
				t.Fatalf("missing or altered direct/inherited reference %s: %v", mapping.Output, err)
			}
		}
	}
}
func TestMetadataAndOutputGuards(t *testing.T) {
	for _, b := range []string{"# no metadata", "---\nname: bad/name\ndescription: x\n---\n", "---\nname: valid\ndescription: 123\n---\n"} {
		if ValidateMetadata([]byte(b)) == nil {
			t.Fatal("invalid metadata accepted")
		}
	}
	if e := ValidateMetadata([]byte("---\nname: valid-name\ndescription: useful source\n---\n\n# Source\n")); e != nil {
		t.Fatal(e)
	}
	store, e := cache.New(filepath.Join(t.TempDir(), "cache"))
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	link := filepath.Join(dir, "link")
	if e = os.Symlink(t.TempDir(), link); e != nil {
		t.Skip(e)
	}
	if _, e = Generate(context.Background(), planning.Report{}, store, []string{"role"}, Options{Out: link}); e == nil {
		t.Fatal("symlink root accepted")
	}
	if _, e = Build(context.Background(), planning.Report{}, store, []string{"../../escape"}, Options{}); e == nil {
		t.Fatal("unsafe role accepted")
	}
}
