package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/codyconfer/pockit/internal/cache"
	"github.com/codyconfer/pockit/internal/inheritance"
	"github.com/codyconfer/pockit/internal/planning"
)

func TestPackageAdapters(t *testing.T) {
	for _, test := range []struct {
		kind         PackageType
		host, prefix string
		opts         Options
		artifacts    func([]string, string, string) []Artifact
		installation func(string, string) (*Installation, error)
		scoped       bool
	}{
		{PiPackage, "Pi", "", Options{PiExtension: true}, piArtifacts, func(out, _ string) (*Installation, error) { return piInstallation(out) }, true},
		{OpenCodePackage, "OpenCode", "", Options{OpenCodePlugin: true}, openCodeArtifacts, func(out, _ string) (*Installation, error) { return openCodeInstallation(out) }, true},
		{GoosePackage, "Goose", "skills/", Options{GoosePlugin: true}, gooseArtifacts, gooseInstallation, false},
		{CursorPackage, "Cursor", "skills/", Options{CursorPlugin: true}, cursorArtifacts, cursorInstallation, false},
		{ClaudeCodePackage, "Claude Code", "skills/", Options{ClaudeCodePlugin: true}, claudeCodeArtifacts, claudeCodeInstallation, false},
		{CodexPackage, "Codex", "skills/", Options{CodexPlugin: true}, codexArtifacts, codexInstallation, false},
	} {
		t.Run(string(test.kind), func(t *testing.T) {
			a, err := PackageAdapterFor(test.opts)
			if err != nil || a == nil {
				t.Fatalf("selection: %v %v", a, err)
			}
			if a.Type() != test.kind || a.Host() != test.host || a.SkillPath("writer/SKILL.md") != test.prefix+"writer/SKILL.md" {
				t.Fatalf("wrong host/layout: %+v", a)
			}
			if err := a.ValidateName("team-docs"); err != nil {
				t.Fatal(err)
			}
			if (a.ValidateName("@team/docs") == nil) != test.scoped {
				t.Fatal("host name constraints changed")
			}
			if a.ValidateName("../escape") == nil {
				t.Fatal("unsafe name accepted")
			}
			name, version, err := packageIdentity(a, "", "")
			if err != nil || name != DefaultPackageName || version != DefaultPackageVersion {
				t.Fatalf("defaults: %q %q %v", name, version, err)
			}
			if _, _, err := packageIdentity(a, "team-docs", "v1"); err == nil {
				t.Fatal("invalid version accepted")
			}
			roles := []string{"writer"}
			if !reflect.DeepEqual(a.Artifacts(roles, name, version), test.artifacts(roles, name, version)) {
				t.Fatal("host artifact bytes changed")
			}
			out := t.TempDir()
			got, err := a.Installation(out, name)
			want, wantErr := test.installation(out, name)
			if err != nil || wantErr != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("installation changed: %+v %+v %v %v", got, want, err, wantErr)
			}

			plan := planning.Report{Entries: []planning.Entry{{ID: "id", URL: "https://example.com/doc", Mappings: []planning.Mapping{{Context: inheritance.Context{Role: "writer"}, Action: planning.Pointer, Output: "writer/sources.json"}}}}}
			r, err := Build(context.Background(), plan, &cache.Store{}, roles, test.opts)
			if err != nil {
				t.Fatal(err)
			}
			repeated, err := Build(context.Background(), plan, &cache.Store{}, roles, test.opts)
			if err != nil || !reflect.DeepEqual(r, repeated) {
				t.Fatal("build is not deterministic")
			}
			var manifest []byte
			for _, artifact := range r.Artifacts {
				if artifact.Bytes != len(artifact.data) {
					t.Fatalf("invalid byte count: %s", artifact.Path)
				}
				if artifact.Path == a.SkillPath("writer/sources.json") {
					manifest = artifact.data
				}
			}
			if !json.Valid(manifest) || !bytes.Contains(manifest, []byte(`"output": "`+test.prefix+`writer/sources.json"`)) {
				t.Fatalf("mapping layout: %s", manifest)
			}
			if plan.Entries[0].Mappings[0].Output != "writer/sources.json" {
				t.Fatal("input plan was mutated")
			}
		})
	}
}

func TestPackageAdapterSelection(t *testing.T) {
	// Check every combination of package flags and AGENT.md.
	for mask := 0; mask < 128; mask++ {
		opts := Options{PiExtension: mask&1 != 0, OpenCodePlugin: mask&2 != 0, GoosePlugin: mask&4 != 0, CursorPlugin: mask&8 != 0, AgentMD: mask&16 != 0, ClaudeCodePlugin: mask&32 != 0, CodexPlugin: mask&64 != 0}
		modes := 0
		for bits := mask; bits != 0; bits &= bits - 1 {
			modes++
		}
		a, err := PackageAdapterFor(opts)
		if (err != nil) != (modes > 1) {
			t.Fatalf("mask=%d adapter=%v err=%v", mask, a, err)
		}
		if modes <= 1 && (a == nil) != (mask == 0 || opts.AgentMD) {
			t.Fatalf("mask=%d wrong adapter: %v", mask, a)
		}
	}
	for _, identity := range [][2]string{{"team-docs", ""}, {"", "2.0.0"}} {
		if _, _, err := packageIdentity(nil, identity[0], identity[1]); err == nil {
			t.Fatal("plain output accepted package metadata")
		}
	}
	a, _ := PackageAdapterFor(Options{CursorPlugin: true})
	for _, name := range []string{"@team/docs", "team.docs", strings.Repeat("a", 65)} {
		if _, _, err := packageIdentity(a, name, ""); err == nil {
			t.Fatalf("invalid Cursor name accepted: %q", name)
		}
	}
}
