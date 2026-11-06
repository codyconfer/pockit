package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOfflineBundleCheckCLI(t *testing.T) {
	dir := t.TempDir()
	args := []string{"skills", "generate", "--json", "--out", filepath.Join(dir, "out"), "--cache", filepath.Join(dir, "cache")}
	for name, body := range map[string]string{"catalog": `{"custom":{"resources":[]}}`, "schema": `{"type":"object"}`, "inheritance": `{"custom":[]}`} {
		path := filepath.Join(dir, name+".json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		args = append(args, "--"+name, path)
	}
	for _, step := range []struct {
		flags []string
		code  int
		dirty bool
	}{
		{[]string{"--check"}, Partial, true},
		{nil, Success, false},
		{[]string{"--check"}, Success, false},
		{[]string{"--check", "--overwrite"}, Invalid, false},
		{[]string{"--check", "--dry-run"}, Invalid, false},
	} {
		var out, diag bytes.Buffer
		if code := Run(context.Background(), append(append([]string{}, args...), step.flags...), &out, &diag); code != step.code {
			t.Fatalf("code=%d flags=%v: %s", code, step.flags, diag.String())
		}
		var report struct {
			NeedsRebuild bool `json:"needs_rebuild"`
			Written      int  `json:"written"`
		}
		if err := json.Unmarshal(out.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		if report.NeedsRebuild != step.dirty {
			t.Fatalf("wrong staleness: %s", out.String())
		}
		if len(step.flags) > 0 && report.Written != 0 {
			t.Fatal("check wrote artifacts")
		}
	}
}
