package skills

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/codyconfer/pockit/internal/cache"
	"github.com/codyconfer/pockit/internal/catalog"
	"github.com/codyconfer/pockit/internal/inheritance"
	"github.com/codyconfer/pockit/internal/planning"
)

func TestInvalidSourceSkillMetadataIsNotRewrittenOrIncluded(t *testing.T) {
	c, e := catalog.Parse([]byte(`{"role":{"resources":[{"url":"https://example.com/SKILL.md","kind":"gh_file_md"}]}}`), []byte(`{"type":"object"}`))
	if e != nil {
		t.Fatal(e)
	}
	resolved, e := inheritance.Resolve(c, inheritance.Graph{"role": nil}, "role")
	if e != nil {
		t.Fatal(e)
	}
	store, e := cache.New(filepath.Join(t.TempDir(), "cache"))
	if e != nil {
		t.Fatal(e)
	}
	plan := planning.Build(resolved, "role", store.Status)
	id := plan.Entries[0].ID
	for _, body := range []string{"# missing metadata\n", "---\nname: original-source\ndescription: Original source description\n---\n# Original bytes\n"} {
		if e := store.Write(id, cache.Record{URL: plan.Entries[0].URL, Outcome: "success", MIME: "text/markdown"}, []byte(body)); e != nil {
			t.Fatal(e)
		}
		report, e := Build(context.Background(), plan, store, []string{"role"}, Options{DryRun: true})
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, artifact := range report.Artifacts {
			if filepath.Dir(artifact.Path) == "role/references" {
				found = true
				if string(artifact.data) != body {
					t.Fatal("source metadata/bytes changed")
				}
			}
		}
		valid := ValidateMetadata([]byte(body)) == nil
		if found != valid || (report.Failed == 0) != valid {
			t.Fatalf("invalid source disposition: %+v", report.Results)
		}
	}
}
