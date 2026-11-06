package cli

import (
	"reflect"
	"testing"

	"github.com/codyconfer/pockit/internal/skills"
)

type unknownPackageAdapter struct{ skills.PackageAdapter }

func (unknownPackageAdapter) Type() skills.PackageType { return "unknown" }

func TestPackageOutputAdapters(t *testing.T) {
	installation := &skills.Installation{Command: "pi install '/bundle'", Note: "review first", Config: []byte(`{"plugin":["file:///bundle/index.js"]}`)}
	for _, test := range []struct {
		name                       string
		opts                       skills.Options
		instructions               []string
		failureHelp, installedNote bool
	}{
		{"Pi", skills.Options{PiExtension: true}, []string{"Install in Pi: " + installation.Command}, false, false},
		{"OpenCode", skills.Options{OpenCodePlugin: true}, []string{"Install in OpenCode: review first", string(installation.Config)}, true, false},
		{"Goose", skills.Options{GoosePlugin: true}, []string{"Install in Goose: review first"}, false, false},
		{"Cursor", skills.Options{CursorPlugin: true}, []string{"Install in Cursor: review first"}, false, true},
		{"Claude Code", skills.Options{ClaudeCodePlugin: true}, []string{"Install in Claude Code: review first"}, false, true},
		{"Codex", skills.Options{CodexPlugin: true}, []string{"Install in Codex: review first"}, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			adapter, err := skills.PackageAdapterFor(test.opts)
			if err != nil {
				t.Fatal(err)
			}
			output, err := newPackageOutput(adapter)
			if err != nil || output == nil {
				t.Fatalf("adapter missing: %v", err)
			}
			if output.Host() != test.name || output.install == nil {
				t.Fatalf("wrong installer: %+v", output)
			}
			if !reflect.DeepEqual(output.instructions(installation), test.instructions) {
				t.Fatal("manual instructions changed")
			}
			if output.instructionsOnFailure != test.failureHelp || output.noteAfterInstall != test.installedNote {
				t.Fatal("installation diagnostics changed")
			}
		})
	}
	if output, err := newPackageOutput(nil); output != nil || err != nil {
		t.Fatalf("plain output has installer: %v %v", output, err)
	}
	if _, err := newPackageOutput(unknownPackageAdapter{}); err == nil {
		t.Fatal("unknown package host accepted")
	}
}
