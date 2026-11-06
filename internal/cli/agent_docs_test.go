package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codyconfer/pockit/internal/app"
	"github.com/codyconfer/pockit/internal/cache"
	"github.com/codyconfer/pockit/internal/skills"
	"gopkg.in/yaml.v3"
)

// Exercise the nightly recipe offline with fixture bytes, not live upstream fetches.
func TestAgentDocsBundles(t *testing.T) {
	dir := t.TempDir()
	sources := filepath.Join("..", "..", "examples", "agent-docs", "sources.json")
	roles := filepath.Join("..", "..", "examples", "agent-docs", "roles.json")
	cacheDir := filepath.Join(dir, "cache")
	session, err := app.Open(app.Inputs{Catalog: sources, Inheritance: roles, Embedded: true}, cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := session.Plan("")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Entries) != 8 || len(session.Roles("")) != 4 {
		t.Fatalf("unexpected documentation catalog: %+v", plan)
	}
	for _, e := range plan.Entries {
		if !e.Fetchable {
			t.Fatalf("documentation is not fetchable: %s", e.URL)
		}
		if err := session.Store.Write(e.ID, cache.Record{URL: e.URL, MIME: "text/markdown", Outcome: "success"}, []byte("# Fixture\n\n[Upstream](../guide.md)\n")); err != nil {
			t.Fatal(err)
		}
	}
	for _, host := range []string{"pi", "opencode", "goose", "cursor"} {
		flag := "--" + host + "-plugin"
		if host == "pi" {
			flag = "--pi-extension"
		}
		bundle := filepath.Join(dir, host)
		for _, check := range []bool{false, true} {
			args := []string{"skills", "generate", flag, "--sources", sources, "--roles", roles, "--cache", cacheDir, "--out", bundle, "--json"}
			if check {
				args = append(args, "--check")
			}
			var out, diag bytes.Buffer
			if code := Run(context.Background(), args, &out, &diag); code != Success {
				t.Fatalf("%s check=%v code=%d stdout=%s stderr=%s", host, check, code, out.String(), diag.String())
			}
			var report skills.Report
			if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.NeedsRebuild || report.Failed != 0 {
				t.Fatalf("report: %s %v", out.String(), err)
			}
		}
		root := bundle
		if host == "goose" || host == "cursor" {
			root = filepath.Join(root, "skills")
		}
		for _, role := range []string{"pi-docs", "opencode-docs", "goose-docs", "cursor-docs"} {
			wrapper, err := os.ReadFile(filepath.Join(root, role, "SKILL.md"))
			if err != nil || !strings.Contains(string(wrapper), "references/") {
				t.Fatalf("missing references for %s/%s: %v", host, role, err)
			}
			var index []skills.BundleSource
			body, err := os.ReadFile(filepath.Join(root, role, "bundle-index.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(body, &index); err != nil || len(index) != 2 {
				t.Fatalf("index: %s %v", body, err)
			}
			var manifest struct {
				Sources []struct {
					Mappings []struct {
						Output string `json:"output"`
					} `json:"mappings"`
				} `json:"sources"`
			}
			body, err = os.ReadFile(filepath.Join(root, role, "sources.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(body, &manifest); err != nil {
				t.Fatal(err)
			}
			for _, source := range manifest.Sources {
				for _, mapping := range source.Mappings {
					if _, err := os.Stat(filepath.Join(bundle, mapping.Output)); err != nil {
						t.Fatalf("broken bundle mapping %s: %v", mapping.Output, err)
					}
				}
			}
			for _, source := range index {
				if len(source.LocalPaths) != 1 {
					t.Fatalf("missing source: %+v", source)
				}
				body, err := os.ReadFile(filepath.Join(root, role, source.LocalPaths[0]))
				if err != nil || string(body) != "# Fixture\n\n[Upstream](../guide.md)\n" {
					t.Fatalf("source changed: %s %v", body, err)
				}
			}
		}
	}
}

func TestAgentDocsWorkflowShell(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("Bash unavailable; workflow shell syntax check")
	}
	body, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "nightly-agent-docs.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(body, &workflow); err != nil {
		t.Fatal(err)
	}
	steps := workflow.Jobs["rebuild"].Steps
	if len(steps) == 0 {
		t.Fatal("workflow has no rebuild steps")
	}
	found := false
	for _, step := range steps {
		if step.Run == "" {
			continue
		}
		if output, err := exec.Command(bash, "-n", "-c", step.Run).CombinedOutput(); err != nil {
			t.Fatalf("invalid workflow shell: %s %v", output, err)
		}
		if strings.Contains(step.Run, "for host in pi opencode goose cursor; do") {
			found = true
			for _, flag := range []string{"--pi-extension", "--opencode-plugin", "--goose-plugin", "--cursor-plugin", "--check"} {
				if !strings.Contains(step.Run, flag) {
					t.Fatalf("workflow missing %s", flag)
				}
			}
		}
	}
	if !found {
		t.Fatal("workflow does not generate all four hosts")
	}
}
