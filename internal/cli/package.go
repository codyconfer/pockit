package cli

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"

	"github.com/codyconfer/pockit/internal/skills"
)

// packageOutput adapts host installation side effects to a common CLI lifecycle.
// Generation and host metadata stay in skills; this layer owns subprocesses,
// user configuration changes and presentation of installation instructions.
type packageOutput struct {
	skills.PackageAdapter
	install               func(context.Context, string, string, bool, *skills.Installation, io.Writer) (string, error)
	instructions          func(*skills.Installation) []string
	instructionsOnFailure bool
	noteAfterInstall      bool
}

func newPackageOutput(adapter skills.PackageAdapter) (*packageOutput, error) {
	if adapter == nil {
		return nil, nil
	}
	outputs := map[skills.PackageType]packageOutput{
		skills.PiPackage: {
			install:      installPiPackage,
			instructions: func(i *skills.Installation) []string { return []string{"Install in Pi: " + i.Command} },
		},
		skills.OpenCodePackage: {
			install: func(_ context.Context, _, _ string, _ bool, i *skills.Installation, _ io.Writer) (string, error) {
				path, err := installOpenCode(i.Config)
				return installedMessage("OpenCode", path), err
			},
			instructions: func(i *skills.Installation) []string {
				return []string{"Install in OpenCode: " + i.Note, string(i.Config)}
			},
			instructionsOnFailure: true,
		},
		skills.GoosePackage: {
			install:      localPackageInstaller("Goose", installGoose),
			instructions: func(i *skills.Installation) []string { return []string{"Install in Goose: " + i.Note} },
		},
		skills.ClaudeCodePackage: stagedPluginOutput(false),
		skills.CodexPackage:      stagedPluginOutput(true),
		skills.CursorPackage: {
			install:          localPackageInstaller("Cursor", installCursor),
			instructions:     func(i *skills.Installation) []string { return []string{"Install in Cursor: " + i.Note} },
			noteAfterInstall: true,
		},
	}
	output, ok := outputs[adapter.Type()]
	if !ok {
		return nil, fmt.Errorf("unsupported package output %q", adapter.Type())
	}
	output.PackageAdapter = adapter
	return &output, nil
}

func stagedPluginOutput(codex bool) packageOutput {
	opts := skills.Options{ClaudeCodePlugin: !codex, CodexPlugin: codex}
	adapter, _ := skills.PackageAdapterFor(opts)
	return packageOutput{
		install: func(_ context.Context, out, name string, overwrite bool, i *skills.Installation, _ io.Writer) (string, error) {
			path, err := stageSkillPlugin(out, name, overwrite, codex)
			if err != nil {
				return "", err
			}
			instructions, err := adapter.Installation(path, name)
			if err != nil {
				return "", err
			}
			*i = *instructions
			return "Staged for " + adapter.Host() + ": " + path + ". Follow the loading instructions to enable skills.", nil
		},
		instructions:     func(i *skills.Installation) []string { return []string{"Install in " + adapter.Host() + ": " + i.Note} },
		noteAfterInstall: true,
	}
}

func installedMessage(host, path string) string {
	return "Installed in " + host + ": " + path + ". Restart " + host + " to load skills."
}

func localPackageInstaller(host string, install func(string, string, bool) (string, error)) func(context.Context, string, string, bool, *skills.Installation, io.Writer) (string, error) {
	return func(_ context.Context, out, name string, overwrite bool, _ *skills.Installation, _ io.Writer) (string, error) {
		path, err := install(out, name, overwrite)
		return installedMessage(host, path), err
	}
}

func installPiPackage(ctx context.Context, out, _ string, _ bool, _ *skills.Installation, diagnostics io.Writer) (string, error) {
	absolute, err := filepath.Abs(out)
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "pi", "install", absolute)
	cmd.Stdout, cmd.Stderr = diagnostics, diagnostics
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return "Installed in Pi.", nil
}
