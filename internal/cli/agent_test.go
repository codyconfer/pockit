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

func TestAgentMDCLI(t *testing.T) {
	dir := t.TempDir()
	inputs := []string{"--cache", filepath.Join(dir, "cache")}
	for name, body := range map[string]string{"sources": `{"custom":{"resources":[]}}`, "schema": `{"type":"object"}`, "roles": `{"custom":[]}`} {
		path := filepath.Join(dir, name+".json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, "--"+name, path)
	}
	for _, command := range [][]string{{"skills", "generate"}, {"input"}} {
		outDir := filepath.Join(t.TempDir(), "bundle")
		for _, step := range []struct {
			flags   []string
			code    int
			written int
		}{
			{[]string{"--dry-run"}, Success, 0},
			{nil, Success, 1},
			{nil, Refused, 0},
			{[]string{"--overwrite"}, Success, 1},
			{[]string{"--check"}, Success, 0},
		} {
			args := append(append([]string{}, command...), "--agent-md", "--json", "--out", outDir)
			args = append(args, inputs...)
			args = append(args, step.flags...)
			var out, diag bytes.Buffer
			if code := Run(context.Background(), args, &out, &diag); code != step.code {
				t.Fatalf("%v: %d %s %s", args, code, &out, &diag)
			}
			var report skills.Report
			if err := json.Unmarshal(out.Bytes(), &report); err != nil || report.Written != step.written || len(report.Artifacts) != 1 || report.Artifacts[0].Path != "AGENT.md" {
				t.Fatalf("%s %v", &out, err)
			}
		}
	}
	for _, flags := range [][]string{{"--pi-extension"}, {"--opencode-plugin"}, {"--install"}, {"--package-name", "example"}, {"--package-version", "1.0.0"}} {
		args := append([]string{"skills", "generate", "--agent-md", "--json"}, flags...)
		var out, diag bytes.Buffer
		if code := Run(context.Background(), args, &out, &diag); code != Invalid {
			t.Fatalf("%v: %d %s", flags, code, &diag)
		}
	}
}
