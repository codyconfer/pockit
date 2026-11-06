package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"
)

func TestDefaultProgressCommands(t *testing.T) {
	for _, test := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"sources", "validate"}, Success, "100%"},
		{[]string{"sources", "plan"}, Success, "100%"},
		{[]string{"sources", "fetch"}, Partial, "100%"},
		{[]string{"skills", "generate", "--out", filepath.Join(t.TempDir(), "skills"), "--dry-run"}, Partial, "100%"},
		{[]string{"skills", "generate", "--out", filepath.Join(t.TempDir(), "pi"), "--pi-extension"}, Partial, "100%"},
		{[]string{"sources", "plan", "--role", "absent"}, Invalid, "0%"},
	} {
		t.Run(test.args[0]+"/"+test.args[1]+"/"+test.want, func(t *testing.T) {
			var out, diag bytes.Buffer
			args := append(append([]string{}, test.args...), fixtureArgs(t)...)
			code := Run(context.Background(), args, &out, &diag)
			if code != test.code {
				t.Fatalf("exit=%d: %s", code, diag.String())
			}
			if !bytes.Contains(out.Bytes(), []byte("[")) || !bytes.Contains(out.Bytes(), []byte(test.want)) || bytes.Count(out.Bytes(), []byte("\n")) != 1 {
				t.Fatalf("bad progress bar: %s", out.String())
			}
			if bytes.Contains(diag.Bytes(), []byte("Review the generated extension first.")) {
				t.Fatal("generation still prints the review reminder")
			}
			if bytes.Contains(diag.Bytes(), []byte("Keep this directory in place")) {
				t.Fatal("generation still prints the installation note")
			}
			for _, line := range bytes.Split(bytes.TrimSpace(diag.Bytes()), []byte("\n")) {
				if slices.Contains(test.args, "--pi-extension") &&
					bytes.HasPrefix(line, []byte("Install in Pi: pi install ")) {
					continue
				}
				if len(line) > 0 && !bytes.HasPrefix(line, []byte("warning: ")) {
					t.Fatalf("non-warning diagnostic: %s", line)
				}
			}
			if test.code != Success && diag.Len() == 0 {
				t.Fatal("missing warning")
			}
			if json.Valid(out.Bytes()) || bytes.Contains(out.Bytes(), []byte{27}) {
				t.Fatalf("redirected default should be readable, no ANSI: %s", out.String())
			}
		})
	}
}

func TestPlainAndJSONModes(t *testing.T) {
	for _, flags := range [][]string{{"--plain"}, {"--json"}, {"--json", "--plain"}} {
		var out, diag bytes.Buffer
		args := append([]string{"sources", "plan"}, flags...)
		args = append(args, fixtureArgs(t)...)
		if code := Run(context.Background(), args, &out, &diag); code != Success {
			t.Fatalf("exit=%d: %s", code, diag.String())
		}
		machine := flags[0] == "--json"
		if json.Valid(out.Bytes()) != machine || bytes.Contains(out.Bytes(), []byte{27}) {
			t.Fatalf("bad mode %v: %s", flags, out.String())
		}
	}
}

func TestNoUICommand(t *testing.T) {
	var out, diag bytes.Buffer
	if code := Run(context.Background(), nil, &out, &diag); code != Success {
		t.Fatal("bare command should show help")
	}
	if bytes.Contains(out.Bytes(), []byte("pockit ui")) {
		t.Fatal("UI still advertised")
	}
	out.Reset()
	if code := Run(context.Background(), []string{"ui"}, &out, &diag); code != Invalid {
		t.Fatal("UI still accepted")
	}
}

func TestFetchConcurrencyBounds(t *testing.T) {
	for _, workers := range []string{"0", "9", "16"} {
		var out, diag bytes.Buffer
		code := Run(context.Background(), []string{"sources", "fetch", "--concurrency", workers}, &out, &diag)
		if code != Invalid || !bytes.Contains(diag.Bytes(), []byte("concurrency must be 1..8")) {
			t.Fatalf("workers=%s exit=%d stderr=%s", workers, code, diag.String())
		}
	}
}

func TestHumanOutputFailure(t *testing.T) {
	var diag bytes.Buffer
	if code := Run(context.Background(), append([]string{"sources", "validate"}, fixtureArgs(t)...), failWriter{}, &diag); code != Partial {
		t.Fatalf("exit=%d", code)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, context.Canceled }
