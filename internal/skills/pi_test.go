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
	"github.com/codyconfer/pockit/internal/planning"
)

func TestPiPackage(t *testing.T) {
	store := &cache.Store{Path: filepath.Join(t.TempDir(), "cache")}
	plan := planning.Report{Version: 1}
	opts := Options{Out: filepath.Join(t.TempDir(), "a 'quoted' bundle"), PiExtension: true}
	ctx := context.Background()
	preview, err := Build(ctx, plan, store, []string{"writer", "analyst"}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Installation != nil {
		t.Fatal("preview claimed installation readiness")
	}
	report, err := Generate(ctx, plan, store, []string{"analyst", "writer"}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if report.Written != 13 || report.Installation == nil {
		t.Fatalf("bad package report: %+v", report)
	}
	if !strings.Contains(report.Installation.Command, "'\"'\"'") || !strings.Contains(report.Installation.Note, "/reload") {
		t.Fatal("missing quoted installation instructions")
	}
	var manifest struct {
		Version string `json:"version"`
		Pi      struct {
			Extensions    []string `json:"extensions"`
			ChangelogPath string   `json:"changelogPath"`
		} `json:"pi"`
	}
	body, err := os.ReadFile(filepath.Join(opts.Out, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &manifest); err != nil || len(manifest.Pi.Extensions) != 1 || manifest.Pi.Extensions[0] != "./index.ts" {
		t.Fatalf("bad manifest: %s", body)
	}
	if manifest.Version != DefaultPackageVersion || manifest.Pi.ChangelogPath != "./CHANGELOG.md" {
		t.Fatalf("missing Pi version/changelog metadata: %s", body)
	}
	entry, err := os.ReadFile(filepath.Join(opts.Out, "index.ts"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"resources_discover", `const roles: string[] = ["analyst","writer"]`, `join(baseDir, role, "SKILL.md")`} {
		if !strings.Contains(string(entry), want) {
			t.Fatalf("entry missing %q", want)
		}
	}
	for i, artifact := range preview.Artifacts {
		if artifact.Path != report.Artifacts[i].Path || !bytes.Equal(artifact.data, report.Artifacts[i].data) {
			t.Fatal("nondeterministic package")
		}
	}
	opts.Out = filepath.Join(t.TempDir(), "selected")
	selected, err := Generate(ctx, plan, store, []string{"writer"}, opts)
	if err != nil || selected.Written != 9 {
		t.Fatalf("selected role generation: %v", err)
	}
	entry, _ = os.ReadFile(filepath.Join(opts.Out, "index.ts"))
	if strings.Contains(string(entry), "analyst") {
		t.Fatal("unselected role registered")
	}
}

func TestPiPackageSafetyAndDryRun(t *testing.T) {
	store := &cache.Store{Path: filepath.Join(t.TempDir(), "cache")}
	ctx := context.Background()
	plan := planning.Report{Version: 1}
	out := filepath.Join(t.TempDir(), "bundle")
	opts := Options{Out: out, PiExtension: true, DryRun: true}
	r, err := Generate(ctx, plan, store, []string{"writer"}, opts)
	if err != nil || r.Written != 0 || r.Installation != nil {
		t.Fatalf("bad dry run: %+v %v", r, err)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("dry run wrote output")
	}
	if err := os.MkdirAll(out, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(out, "package.json")
	if err := os.WriteFile(path, []byte("sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	opts.DryRun = false
	r, err = Generate(ctx, plan, store, []string{"writer"}, opts)
	if !errors.Is(err, ErrRefused) || r.Written != 0 || r.Installation != nil {
		t.Fatalf("overwrite safety failed: %+v %v", r, err)
	}
	if _, err := os.Stat(filepath.Join(out, "index.ts")); !os.IsNotExist(err) {
		t.Fatal("preflight wrote before refusal")
	}
	opts.Overwrite = true
	if _, err := Generate(ctx, plan, store, []string{"writer"}, opts); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "target"), path); err != nil {
		t.Fatal(err)
	}
	if r, err := Generate(ctx, plan, store, []string{"writer"}, opts); !errors.Is(err, ErrRefused) || r.Installation != nil {
		t.Fatalf("symlink accepted: %v", err)
	}
}

func TestDefaultHasNoPiArtifacts(t *testing.T) {
	r, err := Build(context.Background(), planning.Report{}, &cache.Store{}, []string{"writer"}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Artifacts) != 5 || r.Installation != nil {
		t.Fatalf("default output changed: %+v", r)
	}
}
