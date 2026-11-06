package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/codyconfer/pockit/internal/skills"
)

func TestOpenCodePluginCLI(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	inputs := []string{"--cache", filepath.Join(dir, "cache")}
	for name, body := range map[string]string{"catalog": `{"custom":{"resources":[]}}`, "schema": `{"type":"object"}`, "inheritance": `{"custom":[]}`} {
		path := filepath.Join(dir, name+".json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, "--"+name, path)
	}
	for _, mode := range []string{"json", "plain"} {
		t.Run(mode, func(t *testing.T) {
			bundle := filepath.Join(dir, "data", "pockit", "opencode-plugin")
			if mode == "plain" {
				bundle = filepath.Join(dir, "explicit # bundle")
			}
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
			} {
				t.Run(test.name, func(t *testing.T) {
					args := append([]string{"skills", "generate", "--opencode-plugin", "--" + mode, "--package-name", "@team/docs", "--package-version", "1.2.3"}, inputs...)
					if mode == "plain" {
						args = append(args, "--out", bundle)
					}
					args = append(args, test.flags...)
					var out, diag bytes.Buffer
					if code := Run(context.Background(), args, &out, &diag); code != test.code {
						t.Fatalf("code=%d stdout=%s stderr=%s", code, out.String(), diag.String())
					}
					if mode == "json" {
						var report skills.Report
						if err := json.Unmarshal(out.Bytes(), &report); err != nil || (report.Installation != nil) != test.ready {
							t.Fatalf("report: %s %v", out.String(), err)
						}
					}
					if bytes.Contains(diag.Bytes(), []byte("Install in OpenCode:")) != test.ready || bytes.Contains(diag.Bytes(), []byte("Install in Pi:")) {
						t.Fatalf("wrong instructions: %s", diag.String())
					}
					if test.name == "dry" {
						if _, err := os.Stat(bundle); !os.IsNotExist(err) {
							t.Fatal("dry run wrote files")
						}
					}
				})
			}
		})
	}
	for _, flags := range [][]string{
		{"--pi-extension"}, {"--install", "--dry-run"}, {"--install", "--check", "--out", dir}, {"--package-name", "../bad"}, {"--package-version", "v1"},
	} {
		args := append([]string{"skills", "generate", "--opencode-plugin", "--json"}, flags...)
		var out, diag bytes.Buffer
		if code := Run(context.Background(), args, &out, &diag); code != Invalid || !json.Valid(out.Bytes()) {
			t.Fatalf("invalid flags %v: code=%d stdout=%s stderr=%s", flags, code, out.String(), diag.String())
		}
	}
}
