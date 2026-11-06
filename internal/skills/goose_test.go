package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codyconfer/pockit/internal/cache"
	"github.com/codyconfer/pockit/internal/inheritance"
	"github.com/codyconfer/pockit/internal/planning"
)

func TestGoosePackage(t *testing.T) {
	ctx := context.Background()
	opts := Options{Out: filepath.Join(t.TempDir(), "a 'quoted' # bundle"), GoosePlugin: true, PackageName: "team-docs", PackageVersion: "2.1.0-rc.1"}
	preview, err := Build(ctx, planning.Report{}, &cache.Store{}, []string{"writer", "analyst"}, opts)
	if err != nil || preview.Installation != nil {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	r, err := Generate(ctx, planning.Report{}, &cache.Store{}, []string{"analyst", "writer"}, opts)
	if err != nil || r.Written != 12 || r.Installation == nil {
		t.Fatalf("generation: %+v %v", r, err)
	}
	if r.Installation.Command != "" || len(r.Installation.Config) != 0 || !strings.Contains(r.Installation.Note, "team-docs:ROLE") {
		t.Fatalf("installation: %+v", r.Installation)
	}
	for i, a := range preview.Artifacts {
		if a.Path != r.Artifacts[i].Path || !bytes.Equal(a.data, r.Artifacts[i].data) {
			t.Fatal("nondeterministic package")
		}
	}
	body, err := os.ReadFile(filepath.Join(opts.Out, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]string
	if err := json.Unmarshal(body, &manifest); err != nil || manifest["name"] != opts.PackageName || manifest["version"] != opts.PackageVersion {
		t.Fatalf("manifest: %s %v", body, err)
	}
	for _, role := range []string{"writer", "analyst"} {
		body, err := os.ReadFile(filepath.Join(opts.Out, "skills", role, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateMetadata(body); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"index.js", "index.ts", "package.json", "hooks", "writer"} {
		if _, err := os.Stat(filepath.Join(opts.Out, name)); !os.IsNotExist(err) {
			t.Fatalf("unexpected artifact: %s", name)
		}
	}
	opts.Check = true
	checked, err := Generate(ctx, planning.Report{}, &cache.Store{}, []string{"writer", "analyst"}, opts)
	if err != nil || checked.NeedsRebuild || checked.Written != 0 || checked.Installation != nil {
		t.Fatalf("check: %+v %v", checked, err)
	}
	opts.PackageVersion = "2.1.0"
	checked, err = Generate(ctx, planning.Report{}, &cache.Store{}, []string{"writer", "analyst"}, opts)
	if err != nil || !checked.NeedsRebuild {
		t.Fatalf("changed version: %+v %v", checked, err)
	}
}

func TestGoosePackageSafety(t *testing.T) {
	ctx := context.Background()
	opts := Options{Out: filepath.Join(t.TempDir(), "bundle"), GoosePlugin: true, DryRun: true}
	r, err := Generate(ctx, planning.Report{}, &cache.Store{}, []string{"writer"}, opts)
	if err != nil || r.Written != 0 || r.Installation != nil {
		t.Fatalf("dry run: %+v %v", r, err)
	}
	if _, err := os.Stat(opts.Out); !os.IsNotExist(err) {
		t.Fatal("dry run wrote output")
	}
	opts.DryRun = false
	r, err = Generate(ctx, planning.Report{}, &cache.Store{}, []string{"writer"}, opts)
	if err != nil || !strings.Contains(r.Installation.Note, DefaultPackageName+":ROLE") {
		t.Fatalf("default generation: %+v %v", r, err)
	}
	r, err = Generate(ctx, planning.Report{}, &cache.Store{}, []string{"writer"}, opts)
	if !errors.Is(err, ErrRefused) || r.Written != 0 || r.Installation != nil {
		t.Fatalf("overwrite allowed: %+v %v", r, err)
	}
	opts.Overwrite = true
	if _, err := Generate(ctx, planning.Report{}, &cache.Store{}, []string{"writer"}, opts); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(opts.Out, "skills", "writer", "SKILL.md")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "target"), path); err != nil {
		t.Fatal(err)
	}
	r, err = Generate(ctx, planning.Report{}, &cache.Store{}, []string{"writer"}, opts)
	if !errors.Is(err, ErrRefused) || r.Installation != nil {
		t.Fatalf("symlink allowed: %+v %v", r, err)
	}
	for _, opts := range []Options{
		{GoosePlugin: true, PiExtension: true}, {GoosePlugin: true, OpenCodePlugin: true}, {GoosePlugin: true, AgentMD: true},
		{GoosePlugin: true, PackageName: "@team/docs"}, {GoosePlugin: true, PackageName: "../bad"}, {GoosePlugin: true, PackageVersion: "v1"},
	} {
		if _, err := Build(ctx, planning.Report{}, &cache.Store{}, []string{"writer"}, opts); err == nil {
			t.Fatalf("invalid options accepted: %+v", opts)
		}
	}
}

func TestGooseMappingPaths(t *testing.T) {
	plan := planning.Report{Entries: []planning.Entry{{ID: "id", URL: "https://example.com/doc", Mappings: []planning.Mapping{{Context: inheritance.Context{Role: "writer"}, Action: planning.Pointer, Output: "writer/sources.json"}}}}}
	r, err := Build(context.Background(), plan, &cache.Store{}, []string{"writer"}, Options{GoosePlugin: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range r.Artifacts {
		if a.Path == "skills/writer/sources.json" {
			var manifest struct {
				Sources []planning.Entry `json:"sources"`
			}
			if err := json.Unmarshal(a.data, &manifest); err != nil {
				t.Fatal(err)
			}
			if got := manifest.Sources[0].Mappings[0].Output; got != a.Path {
				t.Fatalf("mapping=%s want=%s", got, a.Path)
			}
		}
	}
	if plan.Entries[0].Mappings[0].Output != "writer/sources.json" {
		t.Fatal("mutated input plan")
	}
}
