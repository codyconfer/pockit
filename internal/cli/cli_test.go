package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOfflineCLI(t *testing.T) {
	for _, args := range [][]string{{"sources", "validate", "--json"}, {"sources", "plan", "--role", "as-reviewer", "--json"}} {
		var out, diag bytes.Buffer
		if code := Run(context.Background(), args, &out, &diag); code != Success {
			t.Fatalf("code=%d: %s", code, diag.String())
		}
		if !json.Valid(out.Bytes()) || bytes.Contains(out.Bytes(), []byte{27}) || diag.Len() != 0 {
			t.Fatalf("bad machine output %s stderr %s", out.String(), diag.String())
		}
	}
	var out, diag bytes.Buffer
	if Run(context.Background(), []string{"sources", "plan", "--role", "absent"}, &out, &diag) != Invalid {
		t.Fatal("unknown role exit")
	}
	dir := t.TempDir()
	catalog := filepath.Join(dir, "catalog.json")
	schema := filepath.Join(dir, "schema.json")
	roles := filepath.Join(dir, "roles.json")
	for path, content := range map[string]string{catalog: `{"custom":{"resources":[]}}`, schema: `{"type":"object"}`, roles: `{"custom":[]}`} {
		if e := os.WriteFile(path, []byte(content), 0600); e != nil {
			t.Fatal(e)
		}
	}
	out.Reset()
	diag.Reset()
	if Run(context.Background(), []string{"sources", "validate", "--json", "--catalog", catalog, "--schema", schema, "--inheritance", roles}, &out, &diag) != Success {
		t.Fatal(diag.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("custom")) {
		t.Fatal("ignored overrides")
	}
}

func TestPiExtensionCLI(t *testing.T) {
	dir := t.TempDir()
	inputs := []string{}
	for name, content := range map[string]string{"catalog": `{"custom":{"resources":[]}}`, "schema": `{"type":"object"}`, "inheritance": `{"custom":[]}`} {
		path := filepath.Join(dir, name+".json")
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, "--"+name, path)
	}
	for _, test := range []struct {
		name string
		dry  bool
		code int
	}{
		{"dry", true, Success}, {"write", false, Success}, {"refused", false, Refused},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"skills", "generate", "--json", "--pi-extension", "--out", filepath.Join(dir, "bundle"), "--cache", filepath.Join(dir, "cache")}, inputs...)
			if test.dry {
				args = append(args, "--dry-run")
			}
			var out, diag bytes.Buffer
			if code := Run(context.Background(), args, &out, &diag); code != test.code {
				t.Fatalf("code %d: %s", code, diag.String())
			}
			if !json.Valid(out.Bytes()) {
				t.Fatalf("not JSON: %s", out.String())
			}
			hasInstall := bytes.Contains(out.Bytes(), []byte(`"installation"`))
			if hasInstall != (test.name == "write") {
				t.Fatalf("wrong install readiness: %s", out.String())
			}
			if test.name == "write" && !bytes.Contains(diag.Bytes(), []byte("pi install ")) {
				t.Fatalf("no printed install command: %s", diag.String())
			}
		})
	}
}

func TestHumanPiPackagePrintsInstall(t *testing.T) {
	for _, mode := range []string{"default", "plain"} {
		t.Run(mode, func(t *testing.T) {
			bundle := filepath.Join(t.TempDir(), "bundle with spaces")
			args := []string{"skills", "generate", "--pi-extension", "--out", bundle, "--cache", filepath.Join(t.TempDir(), "cache")}
			if mode == "plain" {
				args = append(args, "--plain")
			}
			for _, test := range []struct {
				name string
				dry  bool
				code int
			}{
				{"dry", true, Partial}, {"write", false, Partial}, {"refused", false, Refused},
			} {
				t.Run(test.name, func(t *testing.T) {
					runArgs := append([]string{}, args...)
					if test.dry {
						runArgs = append(runArgs, "--dry-run")
					}
					var out, diag bytes.Buffer
					code := Run(context.Background(), runArgs, &out, &diag)
					if code != test.code {
						t.Fatalf("code=%d stdout=%s stderr=%s", code, out.String(), diag.String())
					}
					want := "Install in Pi: pi install '" + bundle + "'"
					if hasInstall := bytes.Contains(diag.Bytes(), []byte(want)); hasInstall != (test.name == "write") {
						t.Fatalf("wrong install instructions: %s", diag.String())
					}
				})
			}
		})
	}
}

func TestPartialPiPackagePrintsInstall(t *testing.T) {
	var out, diag bytes.Buffer
	code := Run(context.Background(), []string{"skills", "generate", "--json", "--pi-extension", "--out", filepath.Join(t.TempDir(), "bundle"), "--cache", filepath.Join(t.TempDir(), "cache")}, &out, &diag)
	if code != Partial || !json.Valid(out.Bytes()) || !bytes.Contains(out.Bytes(), []byte(`"installation"`)) || !bytes.Contains(diag.Bytes(), []byte("pi install ")) {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, out.String(), diag.String())
	}
}
