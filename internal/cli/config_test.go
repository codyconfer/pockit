package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/codyconfer/pockit/data"
	"github.com/codyconfer/pockit/internal/config"
)

func TestConfigCommands(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	run := func(args []string, want int) []byte {
		t.Helper()
		var out, diag bytes.Buffer
		if code := Run(context.Background(), args, &out, &diag); code != want {
			t.Fatalf("%v: code=%d stdout=%s stderr=%s", args, code, &out, &diag)
		}
		return out.Bytes()
	}
	if got := run([]string{"config", "schema"}, Success); !bytes.Equal(got, data.Schema) {
		t.Fatal("stdout schema differs from bundled schema")
	}
	path := filepath.Join(t.TempDir(), "schema.json")
	run([]string{"config", "schema", "--out", path}, Success)
	run([]string{"config", "schema", "--out", path}, Refused)
	run([]string{"config", "schema", "--out", path, "--overwrite"}, Success)
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, data.Schema) {
		t.Fatal("file schema differs", err)
	}
	if !json.Valid(run([]string{"config", "init", "--json"}, Success)) {
		t.Fatal("invalid init report")
	}
	run([]string{"config", "init"}, Refused)
	run([]string{"config", "init", "--overwrite"}, Success)
	dir, _ := config.Dir()
	if err := os.WriteFile(filepath.Join(dir, "sources.json"), []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	run([]string{"sources", "validate", "--json"}, Invalid)
	run([]string{"config", "schema"}, Success)
	run([]string{"config", "init", "--overwrite"}, Success)
	run([]string{"sources", "validate", "--json"}, Success)
	for _, args := range [][]string{{"config"}, {"config", "absent"}, {"config", "init", "--out", path}, {"config", "schema", "extra"}, {"config", "schema", "--absent"}} {
		run(args, Invalid)
	}
	for _, command := range []string{"init", "schema"} {
		run([]string{"config", command, "--help"}, Success)
	}
	var diag bytes.Buffer
	if code := Run(context.Background(), []string{"config", "schema"}, failWriter{}, &diag); code != Partial {
		t.Fatalf("failed schema output: %d", code)
	}
}

func TestAdhocInput(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := config.Install(false); err != nil {
		t.Fatal(err)
	}
	dir, _ := config.Dir()
	path := filepath.Join(dir, "sources.json")
	if err := os.WriteFile(path, []byte("invalid installed config"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"input"}, {"input", "--sources", "missing.json"}, {"input", "--roles", "missing.json"}} {
		var out, diag bytes.Buffer
		if code := Run(context.Background(), args, &out, &diag); code != Invalid {
			t.Fatalf("missing required files: %v code=%d", args, code)
		}
	}
	inputs := fixtureArgs(t)
	outDir := filepath.Join(t.TempDir(), "bundle")
	args := []string{"input", "--json", "--out", outDir, "--pi-extension", "--dry-run"}
	for i := range inputs {
		switch inputs[i] {
		case "--catalog":
			inputs[i] = "--sources"
		case "--inheritance":
			inputs[i] = "--roles"
		}
	}
	args = append(args, inputs...)
	var out, diag bytes.Buffer
	if code := Run(context.Background(), args, &out, &diag); code != Partial || !json.Valid(out.Bytes()) {
		t.Fatalf("ad-hoc dry run: code=%d stdout=%s stderr=%s", code, &out, &diag)
	}
	if _, err := os.Stat(outDir); !os.IsNotExist(err) {
		t.Fatal("dry run wrote bundle", err)
	}
	for i, arg := range args {
		if arg == "--dry-run" {
			args = append(args[:i], args[i+1:]...)
			break
		}
	}
	out.Reset()
	diag.Reset()
	if code := Run(context.Background(), args, &out, &diag); code != Partial || !json.Valid(out.Bytes()) {
		t.Fatalf("ad-hoc generation: code=%d stdout=%s stderr=%s", code, &out, &diag)
	}
	for _, name := range []string{"custom/SKILL.md", "package.json", "index.ts"} {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			t.Fatal("bundle artifact missing", name, err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "invalid installed config" {
		t.Fatal("ad-hoc input changed user config", err)
	}
	var help bytes.Buffer
	if code := Run(context.Background(), []string{"input", "--help"}, &help, &diag); code != Success {
		t.Fatal("input help failed")
	}
}
