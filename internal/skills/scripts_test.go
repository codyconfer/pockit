package skills

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codyconfer/pockit/internal/cache"
	"github.com/codyconfer/pockit/internal/inheritance"
	"github.com/codyconfer/pockit/internal/planning"
)

func TestScriptsPackagedForEveryRole(t *testing.T) {
	store, err := cache.New(filepath.Join(t.TempDir(), "cache"))
	if err != nil {
		t.Fatal(err)
	}
	raw := "https://github.com/example/repo/blob/main/generate.sh"
	id := planning.ID(raw)
	body := []byte("#!/bin/sh\nprintf 'agent tool\\n'\n")
	if err := store.Write(id, cache.Record{URL: raw, Outcome: "success", MIME: "text/plain"}, body); err != nil {
		t.Fatal(err)
	}
	roles := []string{"base", "child"}
	e := planning.Entry{ID: id, URL: raw, Fetchable: true}
	for _, role := range roles {
		e.Mappings = append(e.Mappings, planning.Mapping{Action: planning.Script, Context: inheritance.Context{Role: role}, Output: role + "/" + planning.ScriptPath(id, raw)})
	}
	plan := planning.Report{Entries: []planning.Entry{e}}
	report, err := Build(context.Background(), plan, store, roles, Options{})
	if err != nil || report.Failed != 0 {
		t.Fatalf("%+v %v", report, err)
	}
	for _, role := range roles {
		script, linked := false, false
		for _, a := range report.Artifacts {
			if a.Path == role+"/scripts/"+id+".sh" {
				script = bytes.Equal(a.data, body)
			}
			if a.Path == role+"/SKILL.md" {
				linked = strings.Contains(string(a.data), "(scripts/"+id+".sh)")
			}
		}
		if !script || !linked {
			t.Fatalf("script missing or unlinked in %s", role)
		}
		prerequisites := false
		for _, a := range report.Artifacts {
			if a.Path == role+"/scripts/README.md" {
				prerequisites = strings.Contains(string(a.data), "changing the working directory alone") &&
					strings.Contains(string(a.data), "explicit operational") &&
					strings.Contains(string(a.data), "../bundle-index.json")
			}
		}
		if !prerequisites {
			t.Fatalf("script prerequisites missing in %s", role)
		}
	}
	if err := store.Write(id, cache.Record{URL: raw, Outcome: "failure"}, nil); err != nil {
		t.Fatal(err)
	}
	report, err = Build(context.Background(), plan, store, roles, Options{})
	if err != nil || report.Failed != 2 {
		t.Fatalf("missing cache: %+v %v", report, err)
	}
	for _, a := range report.Artifacts {
		if strings.Contains(a.Path, "/scripts/") {
			t.Fatal("packaged unavailable script")
		}
	}
}
