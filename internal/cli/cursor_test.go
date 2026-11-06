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

func TestCursorPluginCLI(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	inputs := []string{"--cache", filepath.Join(dir, "cache")}
	for name, body := range map[string]string{"sources": `{"custom":{"resources":[]}}`, "schema": `{"type":"object"}`, "roles": `{"custom":[]}`} {
		path := filepath.Join(dir, name+".json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, "--"+name, path)
	}
	bundle := filepath.Join(dir, "data", "pockit", "cursor-plugin")
	for _, test := range []struct {
		name  string
		flags []string
		code  int
		ready bool
	}{
		{"dry", []string{"--dry-run"}, Success, false},
		{"write", nil, Success, true},
		{"refuse", nil, Refused, false},
		{"overwrite", []string{"--overwrite"}, Success, true},
		{"check", []string{"--check", "--out", bundle}, Success, false},
		{"stale", []string{"--check", "--out", bundle, "--package-version", "2.0.0"}, Partial, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"skills", "generate", "--cursor-plugin", "--json"}, inputs...)
			args = append(args, test.flags...)
			var out, diag bytes.Buffer
			if code := Run(context.Background(), args, &out, &diag); code != test.code {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, &out, &diag)
			}
			var report skills.Report
			if err := json.Unmarshal(out.Bytes(), &report); err != nil || (report.Installation != nil) != test.ready {
				t.Fatalf("report: %s %v", &out, err)
			}
			if strings.Contains(diag.String(), "Install in Cursor:") != test.ready || strings.Contains(diag.String(), "Install in Pi:") {
				t.Fatalf("instructions: %s", &diag)
			}
			if test.name == "dry" {
				if _, err := os.Stat(bundle); !os.IsNotExist(err) {
					t.Fatal("dry run wrote output")
				}
			}
		})
	}
	for _, flags := range [][]string{
		{"--pi-extension"}, {"--opencode-plugin"}, {"--goose-plugin"}, {"--agent-md"},
		{"--install", "--dry-run"}, {"--install", "--check", "--out", dir},
		{"--package-name", "@team/docs"}, {"--package-name", "Uppercase"}, {"--package-name", "../bad"}, {"--package-version", "v1"},
	} {
		var out, diag bytes.Buffer
		args := append([]string{"skills", "generate", "--cursor-plugin", "--json"}, flags...)
		if code := Run(context.Background(), args, &out, &diag); code != Invalid || !json.Valid(out.Bytes()) {
			t.Fatalf("flags=%v code=%d stdout=%s stderr=%s", flags, code, &out, &diag)
		}
	}
	var out, diag bytes.Buffer
	adhoc := filepath.Join(dir, "adhoc")
	args := append([]string{"input", "--cursor-plugin", "--plain", "--out", adhoc}, inputs...)
	if code := Run(context.Background(), args, &out, &diag); code != Success || !strings.Contains(diag.String(), "Install in Cursor:") {
		t.Fatalf("input: code=%d stdout=%s stderr=%s", code, &out, &diag)
	}
	if _, err := os.Stat(filepath.Join(adhoc, ".cursor-plugin", "plugin.json")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallCursorSafety(t *testing.T) {
	for _, test := range []string{"existing", "dest-symlink", "nested-symlink", "source-symlink", "overlap", "invalid-name"} {
		t.Run(test, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("HOME", dir)
			bundle := filepath.Join(dir, "bundle")
			dest := filepath.Join(dir, ".cursor", "plugins", "local", skills.DefaultPackageName)
			for _, path := range []string{bundle, filepath.Dir(dest)} {
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
					t.Fatal(err)
				}
			}
			name := ""
			switch test {
			case "existing", "nested-symlink":
				if err := os.MkdirAll(filepath.Join(dest, ".cursor-plugin"), 0700); err != nil {
					t.Fatal(err)
				}
				if test == "nested-symlink" {
					link(outside, filepath.Join(dest, ".cursor-plugin", "plugin.json"))
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
			if err := os.MkdirAll(filepath.Join(bundle, ".cursor-plugin"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(bundle, ".cursor-plugin", "plugin.json"), []byte("{}"), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := installCursor(bundle, name, test != "existing")
			if err == nil || (test != "invalid-name" && !errors.Is(err, skills.ErrRefused)) {
				t.Fatalf("unsafe installation: %v", err)
			}
			body, err := os.ReadFile(outside)
			if err != nil || string(body) != "unchanged" {
				t.Fatalf("changed symlink target: %s %v", body, err)
			}
		})
	}
}
