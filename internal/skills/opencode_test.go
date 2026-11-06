package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codyconfer/pockit/internal/cache"
	"github.com/codyconfer/pockit/internal/planning"
)

func TestOpenCodePackage(t *testing.T) {
	ctx := context.Background()
	opts := Options{Out: filepath.Join(t.TempDir(), "a 'quoted' # bundle"), OpenCodePlugin: true, PackageName: "@team/docs", PackageVersion: "2.1.0-rc.1"}
	preview, err := Build(ctx, planning.Report{}, &cache.Store{}, []string{"writer", "analyst"}, opts)
	if err != nil || preview.Installation != nil {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	r, err := Generate(ctx, planning.Report{}, &cache.Store{}, []string{"analyst", "writer"}, opts)
	if err != nil || r.Written != 13 || r.Installation == nil || r.Installation.Command != "" {
		t.Fatalf("generation: %+v %v", r, err)
	}
	var config struct {
		Plugin []string `json:"plugin"`
	}
	if err := json.Unmarshal(r.Installation.Config, &config); err != nil || len(config.Plugin) != 1 {
		t.Fatalf("bad config: %s %v", r.Installation.Config, err)
	}
	u, err := url.Parse(config.Plugin[0])
	if err != nil || u.Scheme != "file" || u.Fragment != "" || u.Path != filepath.Join(opts.Out, "index.js") {
		t.Fatalf("bad file URL: %v %v", u, err)
	}
	for i, a := range preview.Artifacts {
		if a.Path != r.Artifacts[i].Path || !bytes.Equal(a.data, r.Artifacts[i].data) {
			t.Fatal("nondeterministic package")
		}
	}
	body, err := os.ReadFile(filepath.Join(opts.Out, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(body, &manifest); err != nil || manifest["name"] != opts.PackageName || manifest["version"] != opts.PackageVersion || manifest["type"] != "module" || manifest["exports"] != "./index.js" || manifest["pi"] != nil {
		t.Fatalf("bad manifest: %s %v", body, err)
	}
	entry, _ := os.ReadFile(filepath.Join(opts.Out, "index.js"))
	if !strings.Contains(string(entry), `const roles = ["analyst","writer"]`) {
		t.Fatalf("bad role list: %s", entry)
	}
	opts.Check = true
	checked, err := Generate(ctx, planning.Report{}, &cache.Store{}, []string{"writer", "analyst"}, opts)
	if err != nil || checked.NeedsRebuild || checked.Installation != nil || checked.Written != 0 {
		t.Fatalf("check: %+v %v", checked, err)
	}
	opts.PackageVersion = "2.1.0"
	checked, err = Generate(ctx, planning.Report{}, &cache.Store{}, []string{"writer", "analyst"}, opts)
	if err != nil || !checked.NeedsRebuild {
		t.Fatalf("version change not detected: %+v %v", checked, err)
	}
}

func TestOpenCodePackageSafety(t *testing.T) {
	ctx := context.Background()
	opts := Options{Out: filepath.Join(t.TempDir(), "bundle"), OpenCodePlugin: true, DryRun: true}
	r, err := Generate(ctx, planning.Report{}, &cache.Store{}, []string{"writer"}, opts)
	if err != nil || r.Written != 0 || r.Installation != nil {
		t.Fatalf("dry run: %+v %v", r, err)
	}
	if _, err := os.Stat(opts.Out); !os.IsNotExist(err) {
		t.Fatal("dry run wrote output")
	}
	opts.DryRun = false
	if _, err := Generate(ctx, planning.Report{}, &cache.Store{}, []string{"writer"}, opts); err != nil {
		t.Fatal(err)
	}
	if r, err := Generate(ctx, planning.Report{}, &cache.Store{}, []string{"writer"}, opts); !errors.Is(err, ErrRefused) || r.Installation != nil || r.Written != 0 {
		t.Fatalf("overwrite allowed: %+v %v", r, err)
	}
	opts.Overwrite = true
	if _, err := Generate(ctx, planning.Report{}, &cache.Store{}, []string{"writer"}, opts); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(opts.Out, "index.js")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "target"), path); err != nil {
		t.Fatal(err)
	}
	if r, err := Generate(ctx, planning.Report{}, &cache.Store{}, []string{"writer"}, opts); !errors.Is(err, ErrRefused) || r.Installation != nil {
		t.Fatalf("symlink allowed: %+v %v", r, err)
	}
	for _, opts := range []Options{
		{PiExtension: true, OpenCodePlugin: true},
		{OpenCodePlugin: true, PackageName: "../bad"},
		{OpenCodePlugin: true, PackageVersion: "v1"},
	} {
		if _, err := Build(ctx, planning.Report{}, &cache.Store{}, []string{"writer"}, opts); err == nil {
			t.Fatalf("invalid options accepted: %+v", opts)
		}
	}
}

func TestOpenCodePluginRuntime(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js unavailable; optional generated-plugin runtime test")
	}
	out := filepath.Join(t.TempDir(), "bundle # with spaces")
	opts := Options{Out: out, OpenCodePlugin: true}
	if _, err := Generate(context.Background(), planning.Report{}, &cache.Store{}, []string{"writer"}, opts); err != nil {
		t.Fatal(err)
	}
	// Execute the actual generated ESM, not a duplicated implementation.
	script := `
import assert from "node:assert/strict";
import { pathToFileURL } from "node:url";
import { dirname, join } from "node:path";
const entry = process.argv[1];
const module = await import(pathToFileURL(entry).href);
assert.deepEqual(Object.keys(module), ["default"]);
const hooks = await module.default();
assert.deepEqual(Object.keys(hooks), ["config"]);
const selected = join(dirname(entry), "writer");
for (const config of [
  {},
  { skills: { paths: ["existing"], urls: ["https://example.com/skills/"] }, mcp: { untouched: true }, permission: "ask" },
  { skills: ["existing", "https://example.com/skills/"] },
]) {
  const before = structuredClone(config);
  await hooks.config(config);
  const once = structuredClone(config);
  await hooks.config(config);
  assert.deepEqual(config, once);
  if (Array.isArray(config.skills)) {
    assert.deepEqual(config.skills, [...before.skills, selected]);
  } else {
    assert.deepEqual(config.skills.paths, [...(before.skills?.paths ?? []), selected]);
    assert.deepEqual(config.skills.urls, before.skills?.urls);
  }
  assert.deepEqual(config.mcp, before.mcp);
  assert.deepEqual(config.permission, before.permission);
}
`
	cmd := exec.Command(node, "--input-type=module", "-e", script, filepath.Join(out, "index.js"))
	if body, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("plugin runtime: %v\n%s", err, body)
	}
}
