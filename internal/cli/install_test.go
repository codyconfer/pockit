package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallPiExtension(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake pi uses a POSIX shell")
	}
	for _, test := range []struct {
		name    string
		extra   []string
		failure bool
		want    int
		called  bool
	}{
		{name: "install", want: Success, called: true},
		{name: "stable-output", want: Success, called: true},
		{name: "xdg-output", want: Success, called: true},
		{name: "relative-xdg-output", want: Success, called: true},
		{name: "manual-output", want: Success},
		{name: "update", extra: []string{"--overwrite"}, want: Success, called: true},
		{name: "failure", failure: true, want: Partial, called: true},
		{name: "dry", extra: []string{"--dry-run"}, want: Invalid},
		{name: "check", extra: []string{"--check"}, want: Invalid},
		{name: "refused", want: Refused},
		{name: "missing-pi", want: Partial},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("TMPDIR", dir)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
			t.Setenv("HOME", dir)
			t.Setenv("XDG_DATA_HOME", "")
			bundle := filepath.Join(dir, "bundle with ' quotes")
			stable := test.name == "stable-output" || test.name == "xdg-output" || test.name == "relative-xdg-output" || test.name == "manual-output"
			if test.name == "xdg-output" {
				t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
			} else if test.name == "relative-xdg-output" {
				t.Setenv("XDG_DATA_HOME", "relative-data")
			}
			t.Setenv("AppData", filepath.Join(dir, "config"))
			bin := filepath.Join(dir, "bin")
			if err := os.Mkdir(bin, 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin)
			log := filepath.Join(dir, "calls")
			t.Setenv("PI_TEST_CALLS", log)
			script := "#!/bin/sh\n[ -f \"$2/package.json\" ] || exit 9\nprintf '%s\\n' \"$@\" >> \"$PI_TEST_CALLS\"\nprintf 'pi stdout\\n'\nprintf 'pi stderr\\n' >&2\n"
			if test.failure {
				script += "exit 1\n"
			}
			if test.name != "missing-pi" {
				if err := os.WriteFile(filepath.Join(bin, "pi"), []byte(script), 0700); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"skills", "generate", "--json", "--pi-extension", "--out", bundle, "--cache", filepath.Join(dir, "cache")}
			for name, content := range map[string]string{"catalog": `{"custom":{"resources":[]}}`, "schema": `{"type":"object"}`, "inheritance": `{"custom":[]}`} {
				path := filepath.Join(dir, name+".json")
				if err := os.WriteFile(path, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--"+name, path)
			}
			if test.name == "update" || test.name == "refused" {
				var out, diag bytes.Buffer
				if code := Run(context.Background(), args, &out, &diag); code != Success {
					t.Fatalf("prepare: %d %s", code, &diag)
				}
			}
			if stable {
				args = append(args[:4], args[6:]...)
			}
			if test.name != "manual-output" {
				args = append(args, "--install")
			}
			args = append(args, test.extra...)
			var out, diag bytes.Buffer
			if code := Run(context.Background(), args, &out, &diag); code != test.want {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, &out, &diag)
			}
			if !json.Valid(out.Bytes()) || strings.Contains(out.String(), "pi stdout") {
				t.Fatalf("invalid report: %s", &out)
			}
			if stable {
				dataDir := filepath.Join(dir, ".local", "share")
				if test.name == "xdg-output" {
					dataDir = filepath.Join(dir, "data")
				}
				bundle = filepath.Join(dataDir, "pockit", "pi-package")
				info, err := os.Stat(bundle)
				if err != nil || info.Mode().Perm() != 0700 {
					t.Fatalf("stable output not private: %v %v", info, err)
				}
				if _, err := os.Stat(filepath.Join(bundle, "index.ts")); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(diag.String(), bundle) {
					t.Fatalf("missing output path: %s", &diag)
				}
			}
			if test.name == "stable-output" {
				var retryOut, retryDiag bytes.Buffer
				if code := Run(context.Background(), args, &retryOut, &retryDiag); code != Refused {
					t.Fatalf("update without overwrite: %d %s", code, &retryDiag)
				}
				retryOut.Reset()
				retryDiag.Reset()
				updateArgs := append(append([]string(nil), args...), "--overwrite", "--package-version", "1.1.0", "--package-name", "@team/custom-skills")
				if code := Run(context.Background(), updateArgs, &retryOut, &retryDiag); code != Success {
					t.Fatalf("stable update: %d %s", code, &retryDiag)
				}
				manifest, err := os.ReadFile(filepath.Join(bundle, "package.json"))
				if err != nil || !bytes.Contains(manifest, []byte(`"version": "1.1.0"`)) || !bytes.Contains(manifest, []byte(`"name": "@team/custom-skills"`)) {
					t.Fatalf("updated manifest: %s %v", manifest, err)
				}
			}
			calls, err := os.ReadFile(log)
			if test.called {
				wantCalls := "install\n" + bundle + "\n"
				if test.name == "stable-output" {
					wantCalls += wantCalls
				}
				if err != nil || string(calls) != wantCalls {
					t.Fatalf("calls=%q err=%v", calls, err)
				}
				if !strings.Contains(diag.String(), "pi stdout") || !strings.Contains(diag.String(), "pi stderr") {
					t.Fatalf("missing child output: %s", &diag)
				}
			} else if !os.IsNotExist(err) {
				t.Fatalf("unexpected install: %q, %v", calls, err)
			}
			if test.name == "failure" || test.name == "missing-pi" {
				if !strings.Contains(diag.String(), "Pi installation failed") {
					t.Fatalf("missing failure: %s", &diag)
				}
			}
		})
	}
}

func TestInstallRequiresPiExtension(t *testing.T) {
	var out, diag bytes.Buffer
	if code := Run(context.Background(), []string{"skills", "generate", "--install", "--out", t.TempDir()}, &out, &diag); code != Invalid || !strings.Contains(diag.String(), "requires --pi-extension") {
		t.Fatalf("code=%d stderr=%s", code, &diag)
	}
}
