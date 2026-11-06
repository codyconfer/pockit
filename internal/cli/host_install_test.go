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

func TestInstallHostCLI(t *testing.T) {
	for _, host := range []string{"opencode", "goose", "cursor"} {
		t.Run(host, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("HOME", dir)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
			t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
			args := []string{"skills", "generate", "--json", "--" + host + "-plugin", "--install", "--package-name", "custom-skills", "--cache", filepath.Join(dir, "cache")}
			for name, body := range map[string]string{"catalog": `{"custom":{"resources":[]}}`, "schema": `{"type":"object"}`, "inheritance": `{"custom":[]}`} {
				path := filepath.Join(dir, name+".json")
				if err := os.WriteFile(path, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--"+name, path)
			}
			for _, update := range []bool{false, true} {
				if update {
					args = append(args, "--overwrite", "--package-version", "1.1.0")
				}
				var out, diag bytes.Buffer
				if code := Run(context.Background(), args, &out, &diag); code != Success || !json.Valid(out.Bytes()) || !strings.Contains(diag.String(), "Installed in ") {
					t.Fatalf("code=%d stdout=%s stderr=%s", code, &out, &diag)
				}
				if host == "opencode" {
					body, err := os.ReadFile(filepath.Join(dir, "config", "opencode", "opencode.json"))
					var config struct{ Plugin []string }
					if err != nil || json.Unmarshal(body, &config) != nil || len(config.Plugin) != 1 || !strings.HasSuffix(config.Plugin[0], "/opencode-plugin/index.js") {
						t.Fatalf("config=%s err=%v", body, err)
					}
				} else {
					bundle := filepath.Join(dir, "data", "pockit", host+"-plugin")
					installed := filepath.Join(dir, ".agents", "plugins", "custom-skills")
					if host == "cursor" {
						installed = filepath.Join(dir, ".cursor", "plugins", "local", "custom-skills")
					}
					if err := filepath.WalkDir(bundle, func(path string, entry os.DirEntry, err error) error {
						if err != nil || entry.IsDir() {
							return err
						}
						rel, _ := filepath.Rel(bundle, path)
						want, err := os.ReadFile(path)
						if err != nil {
							return err
						}
						got, err := os.ReadFile(filepath.Join(installed, rel))
						if err != nil || !bytes.Equal(got, want) {
							t.Fatalf("copy mismatch: %s %v", rel, err)
						}
						return nil
					}); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}

func TestInstallOpenCodeConfig(t *testing.T) {
	for _, test := range []struct {
		name, body string
		bad        bool
	}{
		{"new", "", false},
		{"existing", `{"model":"test/model","plugin":["other",["configured",{"key":"value"}]],"custom":{"keep":true}}`, false},
		{"modern-key", `{"plugins":["other"],"model":"test/model"}`, false},
		{"both-keys", `{"plugin":["file:///bundle/index.js"],"plugins":["other"]}`, false},
		{"invalid", `{broken`, true},
		{"null", `null`, true},
		{"array", `[]`, true},
		{"invalid-plugin", `{"plugin":"other"}`, true},
		{"null-plugin", `{"plugin":null}`, true},
		{"jsonc", "// comment\n{}", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", dir)
			configDir := filepath.Join(dir, "opencode")
			if err := os.Mkdir(configDir, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(configDir, "opencode.json")
			if test.body != "" {
				if err := os.WriteFile(path, []byte(test.body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			snippet := json.RawMessage(`{"plugin":["file:///bundle/index.js"]}`)
			_, err := installOpenCode(snippet)
			if (err != nil) != test.bad {
				t.Fatalf("err=%v", err)
			}
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if test.bad {
				if string(body) != test.body {
					t.Fatal("failed installation changed config")
				}
				return
			}
			var before, after map[string]json.RawMessage
			_ = json.Unmarshal([]byte(test.body), &before)
			if err := json.Unmarshal(body, &after); err != nil {
				t.Fatal(err)
			}
			for key, value := range before {
				if key == "plugin" || key == "plugins" {
					continue
				}
				var a, b bytes.Buffer
				_ = json.Compact(&a, value)
				_ = json.Compact(&b, after[key])
				if a.String() != b.String() {
					t.Fatalf("changed %s", key)
				}
			}
			if _, err := installOpenCode(snippet); err != nil {
				t.Fatal(err)
			}
			retry, _ := os.ReadFile(path)
			if !bytes.Equal(body, retry) {
				t.Fatal("reinstall changed config")
			}
			if test.name == "existing" && !bytes.Contains(body, []byte(`"configured"`)) {
				t.Fatal("lost configured plugin entry")
			}
		})
	}
}

func TestInstallHostSafety(t *testing.T) {
	for _, test := range []string{"config-symlink", "config-directory", "jsonc-file", "goose-symlink", "goose-existing", "goose-nested-symlink", "source-symlink", "overlap"} {
		t.Run(test, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("HOME", dir)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
			configDir := filepath.Join(dir, "config", "opencode")
			dest := filepath.Join(dir, ".agents", "plugins", skills.DefaultPackageName)
			bundle := filepath.Join(dir, "bundle")
			for _, path := range []string{configDir, filepath.Dir(dest), bundle} {
				if err := os.MkdirAll(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			outside := filepath.Join(dir, "outside")
			if err := os.WriteFile(outside, []byte("unchanged"), 0600); err != nil {
				t.Fatal(err)
			}
			link := func(target, path string) {
				t.Helper()
				if err := os.Symlink(target, path); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			var err error
			switch test {
			case "config-symlink":
				link(outside, filepath.Join(configDir, "opencode.json"))
			case "config-directory":
				err = os.Mkdir(filepath.Join(configDir, "opencode.json"), 0700)
			case "jsonc-file":
				err = os.WriteFile(filepath.Join(configDir, "opencode.jsonc"), []byte("{}"), 0600)
			case "goose-symlink":
				link(bundle, dest)
			case "goose-existing", "goose-nested-symlink":
				err = os.Mkdir(dest, 0700)
				if test == "goose-nested-symlink" {
					link(outside, filepath.Join(dest, "plugin.json"))
				}
			case "source-symlink":
				link(outside, filepath.Join(bundle, "other"))
			case "overlap":
				bundle = filepath.Dir(dest)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(bundle, "plugin.json"), []byte("{}"), 0600); err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(test, "config-") || test == "jsonc-file" {
				_, err = installOpenCode(json.RawMessage(`{"plugin":["file:///bundle/index.js"]}`))
			} else {
				_, err = installGoose(bundle, "", test != "goose-existing")
			}
			if err == nil {
				t.Fatal("unsafe installation succeeded")
			}
			if test != "jsonc-file" && !errors.Is(err, skills.ErrRefused) {
				t.Fatalf("expected safety refusal: %v", err)
			}
			body, _ := os.ReadFile(outside)
			if string(body) != "unchanged" {
				t.Fatal("changed symlink target")
			}
		})
	}
}
