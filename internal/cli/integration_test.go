package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func fixtureArgs(t *testing.T) []string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{"catalog.json": `{"custom":{"resources":[{"url":"https://127.0.0.1/a.md","kind":"gh_file_md"},{"url":"https://example.com/board","kind":"gh_project"},{"url":"https://example.com/mcp","kind":"mcp_endpoint"}]}}`, "schema.json": `{"type":"object"}`, "roles.json": `{"custom":[]}`}
	for name, body := range files {
		if e := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
	}
	return []string{"--catalog", filepath.Join(dir, "catalog.json"), "--schema", filepath.Join(dir, "schema.json"), "--inheritance", filepath.Join(dir, "roles.json"), "--cache", filepath.Join(dir, "cache")}
}
func TestWorkflowExitContracts(t *testing.T) {
	common := fixtureArgs(t)
	outDir := filepath.Join(t.TempDir(), "output")
	cases := []struct {
		args []string
		code int
	}{{[]string{"sources", "validate"}, Success}, {[]string{"sources", "plan", "--role", "custom"}, Success}, {[]string{"sources", "fetch"}, Partial}, {[]string{"skills", "generate", "--out", outDir, "--dry-run"}, Partial}, {[]string{"skills", "generate", "--out", outDir}, Partial}, {[]string{"skills", "generate", "--out", outDir}, Refused}, {[]string{"skills", "generate", "--out", outDir, "--overwrite"}, Partial}, {[]string{"sources", "plan", "--role", "absent"}, Invalid}}
	for _, c := range cases {
		var out, diag bytes.Buffer
		code := Run(context.Background(), append(append(c.args, "--json"), common...), &out, &diag)
		if code != c.code {
			t.Fatalf("%v: code=%d want=%d stderr=%s", c.args, code, c.code, diag.String())
		}
		if !json.Valid(out.Bytes()) || bytes.Contains(out.Bytes(), []byte{27}) {
			t.Fatalf("not clean JSON: %s", out.String())
		}
		if code != Success && diag.Len() == 0 {
			t.Fatal("failure lacks diagnostic")
		}
	}
	var out, diag bytes.Buffer
	if code := Run(context.Background(), []string{"mcp", "serve"}, &out, &diag); code != Invalid || out.Len() != 0 {
		t.Fatal("removed MCP command should be rejected without progress")
	}
	if code := Run(context.Background(), append([]string{"ui"}, common...), &out, &diag); code != Invalid {
		t.Fatal("removed UI command should be rejected")
	}
}
func TestBuiltOfflinePlanOutsideProject(t *testing.T) {
	root, e := filepath.Abs(filepath.Join("..", ".."))
	if e != nil {
		t.Fatal(e)
	}
	binary := filepath.Join(t.TempDir(), "pockit")
	build := exec.Command("go", "build", "-trimpath", "-o", binary, "./cmd/pockit")
	build.Dir = root
	if b, e := build.CombinedOutput(); e != nil {
		t.Fatalf("build: %v %s", e, b)
	}
	cmd := exec.Command(binary, append([]string{"sources", "plan", "--json", "--role", "custom"}, fixtureArgs(t)...)...)
	cmd.Dir = t.TempDir()
	var out, diag bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &diag
	if e := cmd.Run(); e != nil {
		t.Fatalf("offline plan: %v stderr=%s", e, diag.String())
	}
	var report struct {
		Role    string            `json:"role"`
		Entries []json.RawMessage `json:"entries"`
	}
	if e := json.Unmarshal(out.Bytes(), &report); e != nil || report.Role != "custom" || len(report.Entries) != 3 {
		t.Fatalf("plan: %v %s", e, out.String())
	}
}
