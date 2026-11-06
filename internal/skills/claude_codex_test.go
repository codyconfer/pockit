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

func TestClaudeCodeAndCodexPackages(t *testing.T) {
	for _, codex := range []bool{false, true} {
		opts := Options{ClaudeCodePlugin: !codex, CodexPlugin: codex, PackageName: "team-docs", PackageVersion: "2.1.0-rc.1", DryRun: true}
		adapter, _ := PackageAdapterFor(opts)
		t.Run(adapter.Host(), func(t *testing.T) {
			opts.Out = filepath.Join(t.TempDir(), "a 'quoted' # bundle")
			ctx, plan, store := context.Background(), planning.Report{}, &cache.Store{}
			preview, err := Generate(ctx, plan, store, []string{"writer"}, opts)
			if err != nil || preview.Written != 0 || preview.Installation != nil {
				t.Fatalf("preview: %+v %v", preview, err)
			}
			if _, err := os.Stat(opts.Out); !os.IsNotExist(err) {
				t.Fatal("preview wrote files")
			}
			opts.DryRun = false
			r, err := Generate(ctx, plan, store, []string{"writer"}, opts)
			if err != nil || r.Installation == nil || r.Written == 0 {
				t.Fatalf("generate: %+v %v", r, err)
			}
			if !strings.Contains(r.Installation.Command, "'\"'\"'") {
				t.Fatalf("unquoted path: %+v", r.Installation)
			}
			if len(preview.Artifacts) != len(r.Artifacts) {
				t.Fatal("preview differs")
			}
			for i, artifact := range r.Artifacts {
				if artifact.Path != preview.Artifacts[i].Path || !bytes.Equal(artifact.data, preview.Artifacts[i].data) {
					t.Fatal("nondeterministic artifacts")
				}
			}
			manifestPath := ".claude-plugin/plugin.json"
			if codex {
				manifestPath = "plugin.json"
			}
			body, err := os.ReadFile(filepath.Join(opts.Out, manifestPath))
			var manifest map[string]string
			if err != nil || json.Unmarshal(body, &manifest) != nil || manifest["name"] != opts.PackageName || manifest["version"] != opts.PackageVersion {
				t.Fatalf("manifest: %s %v", body, err)
			}
			if codex {
				if manifest["$schema"] != "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json" {
					t.Fatal("missing portable schema")
				}
				body, err := os.ReadFile(filepath.Join(opts.Out, ".agents/plugins/marketplace.json"))
				var marketplace struct {
					Name    string `json:"name"`
					Plugins []struct {
						Name     string            `json:"name"`
						Source   map[string]string `json:"source"`
						Policy   map[string]string `json:"policy"`
						Category string            `json:"category"`
					} `json:"plugins"`
				}
				if err != nil || json.Unmarshal(body, &marketplace) != nil || marketplace.Name != "pockit-team-docs" || len(marketplace.Plugins) != 1 {
					t.Fatalf("marketplace: %s %v", body, err)
				}
				plugin := marketplace.Plugins[0]
				if plugin.Name != opts.PackageName || plugin.Source["source"] != "local" || plugin.Source["path"] != "./" || plugin.Policy["installation"] != "AVAILABLE" || plugin.Policy["authentication"] != "ON_INSTALL" || plugin.Category != "Productivity" {
					t.Fatalf("marketplace entry: %+v", plugin)
				}
			}
			body, err = os.ReadFile(filepath.Join(opts.Out, "skills/writer/SKILL.md"))
			if err != nil || ValidateMetadata(body) != nil {
				t.Fatalf("skill: %s %v", body, err)
			}
			for _, path := range []string{"mcp.json", ".mcp.json", "hooks", "index.js", "package.json", "writer"} {
				if _, err := os.Stat(filepath.Join(opts.Out, path)); !os.IsNotExist(err) {
					t.Fatalf("unexpected active artifact: %s", path)
				}
			}
			opts.Check = true
			checked, err := Generate(ctx, plan, store, []string{"writer"}, opts)
			if err != nil || checked.NeedsRebuild || checked.Installation != nil {
				t.Fatalf("check: %+v %v", checked, err)
			}
			opts.PackageVersion = "2.2.0"
			checked, err = Generate(ctx, plan, store, []string{"writer"}, opts)
			if err != nil || !checked.NeedsRebuild {
				t.Fatalf("stale: %+v %v", checked, err)
			}
			opts.Check = false
			if _, err := Generate(ctx, plan, store, []string{"writer"}, opts); !errors.Is(err, ErrRefused) {
				t.Fatalf("overwrite allowed: %v", err)
			}
			for _, name := range []string{"@team/docs", "Uppercase", "../escape", strings.Repeat("a", 65)} {
				if adapter.ValidateName(name) == nil {
					t.Fatalf("unsafe name accepted: %q", name)
				}
			}
		})
	}
}
