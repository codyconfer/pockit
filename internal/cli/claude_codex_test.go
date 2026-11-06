package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codyconfer/pockit/internal/skills"
)

func TestClaudeCodeAndCodexCLI(t *testing.T) {
	for _, host := range []struct{ flag, name, directory, manifest string }{
		{"claude-code-plugin", "Claude Code", ".claude", ".claude-plugin/plugin.json"},
		{"codex-plugin", "Codex", ".codex", "plugin.json"},
	} {
		t.Run(host.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("HOME", dir)
			t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
			inputs := []string{"--cache", filepath.Join(dir, "cache")}
			for name, body := range map[string]string{"sources": `{"custom":{"resources":[]}}`, "schema": `{"type":"object"}`, "roles": `{"custom":[]}`} {
				path := filepath.Join(dir, name+".json")
				if err := os.WriteFile(path, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
				inputs = append(inputs, "--"+name, path)
			}
			bundle := filepath.Join(dir, "data/pockit", host.flag)
			dest := filepath.Join(dir, host.directory, "plugins/local", skills.DefaultPackageName)
			for _, test := range []struct {
				name  string
				flags []string
				code  int
				ready bool
			}{
				{"dry", []string{"--dry-run"}, Success, false},
				{"write", nil, Success, true},
				{"refuse", nil, Refused, false},
				{"install", []string{"--install", "--overwrite"}, Success, true},
				{"check", []string{"--check", "--out", bundle}, Success, false},
				{"stale", []string{"--check", "--out", bundle, "--package-version", "2.0.0"}, Partial, false},
			} {
				t.Run(test.name, func(t *testing.T) {
					args := append([]string{"skills", "generate", "--" + host.flag, "--json"}, inputs...)
					args = append(args, test.flags...)
					var out, diag bytes.Buffer
					if code := Run(context.Background(), args, &out, &diag); code != test.code {
						t.Fatalf("code=%d out=%s diag=%s", code, &out, &diag)
					}
					var report skills.Report
					if err := json.Unmarshal(out.Bytes(), &report); err != nil || (report.Installation != nil) != test.ready {
						t.Fatalf("report: %s %v", &out, err)
					}
					if test.name == "dry" {
						if _, err := os.Stat(bundle); !os.IsNotExist(err) {
							t.Fatal("preview wrote files")
						}
					} else if test.name == "install" {
						if !strings.Contains(diag.String(), "Staged for "+host.name) || !strings.Contains(diag.String(), dest) {
							t.Fatalf("missing staged instructions: %s", &diag)
						}
						for _, path := range []string{host.manifest, "skills/custom/SKILL.md", "skills/custom/sources.json"} {
							generated, err := os.ReadFile(filepath.Join(bundle, path))
							if err != nil {
								t.Fatal(err)
							}
							installed, err := os.ReadFile(filepath.Join(dest, path))
							if err != nil || !bytes.Equal(generated, installed) {
								t.Fatalf("staged file differs: %s %v", path, err)
							}
						}
					} else if strings.Contains(diag.String(), "Install in "+host.name+":") != test.ready {
						t.Fatalf("instructions: %s", &diag)
					}
				})
			}
			for _, flags := range [][]string{
				{"--pi-extension"}, {"--opencode-plugin"}, {"--goose-plugin"}, {"--cursor-plugin"}, {"--agent-md"},
				{"--install", "--dry-run"}, {"--install", "--check", "--out", dir},
				{"--package-name", "@team/docs"}, {"--package-name", "Uppercase"}, {"--package-name", "../bad"}, {"--package-version", "v1"},
			} {
				var out, diag bytes.Buffer
				args := append([]string{"skills", "generate", "--" + host.flag, "--json"}, flags...)
				if code := Run(context.Background(), args, &out, &diag); code != Invalid || !json.Valid(out.Bytes()) {
					t.Fatalf("flags=%v code=%d out=%s diag=%s", flags, code, &out, &diag)
				}
			}
			var out, diag bytes.Buffer
			adhoc := filepath.Join(dir, "adhoc")
			args := append([]string{"input", "--" + host.flag, "--plain", "--out", adhoc}, inputs...)
			if code := Run(context.Background(), args, &out, &diag); code != Success || !strings.Contains(diag.String(), "Install in "+host.name+":") {
				t.Fatalf("input code=%d out=%s diag=%s", code, &out, &diag)
			}
			if _, err := os.Stat(filepath.Join(adhoc, host.manifest)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStageSkillPluginSafety(t *testing.T) {
	for _, codex := range []bool{false, true} {
		for _, scenario := range []string{"existing", "dest-symlink", "nested-symlink", "source-symlink", "overlap", "invalid-name"} {
			t.Run(scenario+map[bool]string{true: "-codex", false: "-claude"}[codex], func(t *testing.T) {
				dir := t.TempDir()
				t.Setenv("HOME", dir)
				host := ".claude"
				if codex {
					host = ".codex"
				}
				dest := filepath.Join(dir, host, "plugins/local", skills.DefaultPackageName)
				bundle := filepath.Join(dir, "bundle")
				if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(bundle, 0700); err != nil {
					t.Fatal(err)
				}
				outside := filepath.Join(dir, "outside")
				if err := os.WriteFile(outside, []byte("unchanged"), 0600); err != nil {
					t.Fatal(err)
				}
				link := func(target, path string) {
					t.Helper()
					if err := os.Symlink(target, path); err != nil {
						t.Fatal(err)
					}
				}
				name := ""
				switch scenario {
				case "existing", "nested-symlink":
					if err := os.MkdirAll(dest, 0700); err != nil {
						t.Fatal(err)
					}
					if scenario == "nested-symlink" {
						link(outside, filepath.Join(dest, "README.md"))
					}
				case "dest-symlink":
					link(bundle, dest)
				case "source-symlink":
					link(outside, filepath.Join(bundle, "other"))
				case "overlap":
					bundle = filepath.Dir(dest)
				case "invalid-name":
					name = "../escape"
				}
				if err := os.WriteFile(filepath.Join(bundle, "README.md"), []byte("bundle"), 0600); err != nil {
					t.Fatal(err)
				}
				_, err := stageSkillPlugin(bundle, name, scenario != "existing", codex)
				if err == nil || (scenario != "invalid-name" && !errors.Is(err, skills.ErrRefused)) {
					t.Fatalf("unsafe installation: %v", err)
				}
				body, err := os.ReadFile(outside)
				if err != nil || string(body) != "unchanged" {
					t.Fatalf("symlink target changed: %s %v", body, err)
				}
			})
		}
	}
}
