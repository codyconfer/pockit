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

func TestCursorPackage(t *testing.T) {
	ctx := context.Background()
	plan := planning.Report{Entries: []planning.Entry{{ID: "id", URL: "https://example.com/doc", Mappings: []planning.Mapping{{Context: inheritance.Context{Role: "writer"}, Action: planning.Pointer, Output: "writer/sources.json"}}}}}
	opts := Options{Out: filepath.Join(t.TempDir(), "a 'quoted' # bundle"), CursorPlugin: true, PackageName: "team-docs", PackageVersion: "2.1.0-rc.1", DryRun: true}
	preview, err := Generate(ctx, plan, &cache.Store{}, []string{"writer"}, opts)
	if err != nil || preview.Written != 0 || preview.Installation != nil {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	if _, err := os.Stat(opts.Out); !os.IsNotExist(err) {
		t.Fatal("dry run wrote output")
	}
	opts.DryRun = false
	r, err := Generate(ctx, plan, &cache.Store{}, []string{"writer"}, opts)
	if err != nil || r.Written != 8 || r.Installation == nil {
		t.Fatalf("generation: %+v %v", r, err)
	}
	if r.Installation.Command != "" || len(r.Installation.Config) != 0 || !strings.Contains(r.Installation.Note, "~/.cursor/plugins/local/team-docs/") {
		t.Fatalf("installation: %+v", r.Installation)
	}
	for i, a := range preview.Artifacts {
		if a.Path != r.Artifacts[i].Path || !bytes.Equal(a.data, r.Artifacts[i].data) {
			t.Fatal("nondeterministic package")
		}
	}
	body, err := os.ReadFile(filepath.Join(opts.Out, ".cursor-plugin", "plugin.json"))
	var manifest map[string]string
	if err != nil || json.Unmarshal(body, &manifest) != nil || manifest["name"] != opts.PackageName || manifest["version"] != opts.PackageVersion {
		t.Fatalf("manifest: %s %v", body, err)
	}
	body, err = os.ReadFile(filepath.Join(opts.Out, "skills", "writer", "SKILL.md"))
	if err != nil || ValidateMetadata(body) != nil {
		t.Fatalf("skill: %s %v", body, err)
	}
	body, err = os.ReadFile(filepath.Join(opts.Out, "skills", "writer", "sources.json"))
	if err != nil || !bytes.Contains(body, []byte(`"output": "skills/writer/sources.json"`)) || plan.Entries[0].Mappings[0].Output != "writer/sources.json" {
		t.Fatalf("mapping: %s %v", body, err)
	}
	for _, name := range []string{"plugin.json", "mcp.json", "hooks", "index.js", "package.json", "writer"} {
		if _, err := os.Stat(filepath.Join(opts.Out, name)); !os.IsNotExist(err) {
			t.Fatalf("unexpected active artifact: %s", name)
		}
	}
	opts.Check = true
	checked, err := Generate(ctx, plan, &cache.Store{}, []string{"writer"}, opts)
	if err != nil || checked.NeedsRebuild || checked.Installation != nil || checked.Written != 0 {
		t.Fatalf("check: %+v %v", checked, err)
	}
	opts.PackageVersion = "2.2.0"
	checked, err = Generate(ctx, plan, &cache.Store{}, []string{"writer"}, opts)
	if err != nil || !checked.NeedsRebuild {
		t.Fatalf("changed version: %+v %v", checked, err)
	}
	opts.Check = false
	r, err = Generate(ctx, plan, &cache.Store{}, []string{"writer"}, opts)
	if !errors.Is(err, ErrRefused) || r.Installation != nil || r.Written != 0 {
		t.Fatalf("overwrite allowed: %+v %v", r, err)
	}
	opts.Overwrite = true
	path := filepath.Join(opts.Out, ".cursor-plugin", "plugin.json")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "outside"), path); err != nil {
		t.Fatal(err)
	}
	if r, err := Generate(ctx, plan, &cache.Store{}, []string{"writer"}, opts); !errors.Is(err, ErrRefused) || r.Installation != nil {
		t.Fatalf("symlink allowed: %+v %v", r, err)
	}
}

func TestCursorPackageOptions(t *testing.T) {
	for _, opts := range []Options{
		{CursorPlugin: true, PiExtension: true}, {CursorPlugin: true, OpenCodePlugin: true},
		{CursorPlugin: true, GoosePlugin: true}, {CursorPlugin: true, AgentMD: true},
		{CursorPlugin: true, PackageName: "@team/docs"}, {CursorPlugin: true, PackageName: "../bad"},
		{CursorPlugin: true, PackageName: "Uppercase"}, {CursorPlugin: true, PackageName: strings.Repeat("a", 65)},
		{CursorPlugin: true, PackageVersion: "v1"},
	} {
		if _, err := Build(context.Background(), planning.Report{}, &cache.Store{}, []string{"writer"}, opts); err == nil {
			t.Fatalf("invalid options accepted: %+v", opts)
		}
	}
}
