package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageVersionValidation(t *testing.T) {
	for _, test := range []struct {
		name    string
		flags   []string
		message string
	}{
		{"name-requires-extension", []string{"--package-name", "my-skills"}, "--package-name requires --pi-extension"},
		{"invalid-name", []string{"--pi-extension", "--package-name", "../bad"}, "lowercase npm-style name"},
		{"empty-name", []string{"--pi-extension", "--package-name="}, "lowercase npm-style name"},
		{"requires-extension", []string{"--package-version", "1.2.3"}, "requires --pi-extension"},
		{"invalid", []string{"--pi-extension", "--package-version", "v1.2.3"}, "must be SemVer"},
		{"empty", []string{"--pi-extension", "--package-version="}, "must be SemVer"},
	} {
		t.Run(test.name, func(t *testing.T) {
			outDir := filepath.Join(t.TempDir(), "out")
			args := append([]string{"skills", "generate", "--json", "--out", outDir}, test.flags...)
			var out, diag bytes.Buffer
			if code := Run(context.Background(), args, &out, &diag); code != Invalid || !json.Valid(out.Bytes()) || !strings.Contains(diag.String(), test.message) {
				t.Fatalf("code=%d out=%s diag=%s", code, &out, &diag)
			}
			if _, err := os.Stat(outDir); !os.IsNotExist(err) {
				t.Fatalf("invalid input wrote output: %v", err)
			}
		})
	}
}
