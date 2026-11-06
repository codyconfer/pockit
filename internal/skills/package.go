package skills

import "fmt"

// PackageType identifies a host package independently of its command-line flag.
type PackageType string

const (
	PiPackage         PackageType = "pi-package"
	OpenCodePackage   PackageType = "opencode-plugin"
	GoosePackage      PackageType = "goose-plugin"
	CursorPackage     PackageType = "cursor-plugin"
	ClaudeCodePackage PackageType = "claude-code-plugin"
	CodexPackage      PackageType = "codex-plugin"
)

// PackageAdapter adapts a host's layout, metadata and installation description
// to the shared skill builder. Installation side effects belong to the CLI.
// SkillPath accepts and returns slash-separated bundle-relative paths.
type PackageAdapter interface {
	Type() PackageType
	Host() string
	ValidateName(string) error
	SkillPath(string) string
	Artifacts(roles []string, name, version string) []Artifact
	Installation(out, name string) (*Installation, error)
}

// packageAdapter wraps the existing host renderers without coupling the builder
// to their manifest formats or directory layouts.
type packageAdapter struct {
	kind         PackageType
	host         string
	prefix       string
	validate     func(string) error
	artifacts    func([]string, string, string) []Artifact
	installation func(string, string) (*Installation, error)
}

func (a packageAdapter) Type() PackageType              { return a.kind }
func (a packageAdapter) Host() string                   { return a.host }
func (a packageAdapter) ValidateName(name string) error { return a.validate(name) }
func (a packageAdapter) SkillPath(path string) string   { return a.prefix + path }
func (a packageAdapter) Artifacts(roles []string, name, version string) []Artifact {
	return a.artifacts(roles, name, version)
}
func (a packageAdapter) Installation(out, name string) (*Installation, error) {
	return a.installation(out, name)
}

var packageAdapters = map[PackageType]PackageAdapter{
	PiPackage: packageAdapter{
		kind: PiPackage, host: "Pi", validate: ValidatePackageName, artifacts: piArtifacts,
		installation: func(out, _ string) (*Installation, error) { return piInstallation(out) },
	},
	OpenCodePackage: packageAdapter{
		kind: OpenCodePackage, host: "OpenCode", validate: ValidatePackageName, artifacts: openCodeArtifacts,
		installation: func(out, _ string) (*Installation, error) { return openCodeInstallation(out) },
	},
	GoosePackage: packageAdapter{
		kind: GoosePackage, host: "Goose", prefix: "skills/", validate: validateGoosePackageName,
		artifacts: gooseArtifacts, installation: gooseInstallation,
	},
	ClaudeCodePackage: packageAdapter{
		kind: ClaudeCodePackage, host: "Claude Code", prefix: "skills/", validate: validateClaudeCodePackageName,
		artifacts: claudeCodeArtifacts, installation: claudeCodeInstallation,
	},
	CodexPackage: packageAdapter{
		kind: CodexPackage, host: "Codex", prefix: "skills/", validate: validateCodexPackageName,
		artifacts: codexArtifacts, installation: codexInstallation,
	},
	CursorPackage: packageAdapter{
		kind: CursorPackage, host: "Cursor", prefix: "skills/", validate: ValidateCursorPackageName,
		artifacts: cursorArtifacts, installation: cursorInstallation,
	},
}

// PackageAdapterFor resolves legacy flag options at the boundary. A nil adapter
// means plain skills or AGENT.md output. Conflicting modes are always rejected.
func PackageAdapterFor(opts Options) (PackageAdapter, error) {
	var selected PackageAdapter
	modes := 0
	for _, mode := range []struct {
		enabled bool
		kind    PackageType
	}{
		{opts.PiExtension, PiPackage}, {opts.OpenCodePlugin, OpenCodePackage},
		{opts.GoosePlugin, GoosePackage}, {opts.CursorPlugin, CursorPackage},
		{opts.ClaudeCodePlugin, ClaudeCodePackage}, {opts.CodexPlugin, CodexPackage},
	} {
		if mode.enabled {
			modes++
			selected = packageAdapters[mode.kind]
		}
	}
	if opts.AgentMD {
		modes++
	}
	if modes > 1 {
		return nil, fmt.Errorf("Pi extension, OpenCode plugin, Goose plugin, Cursor plugin, Claude Code plugin, Codex plugin and AGENT.md output are mutually exclusive")
	}
	return selected, nil
}

func packageIdentity(adapter PackageAdapter, name, version string) (string, string, error) {
	if adapter == nil {
		if name != "" {
			return "", "", fmt.Errorf("package name requires Pi extension, OpenCode plugin, Goose plugin, Cursor plugin, Claude Code plugin or Codex plugin output")
		}
		if version != "" {
			return "", "", fmt.Errorf("package version requires Pi extension, OpenCode plugin, Goose plugin, Cursor plugin, Claude Code plugin or Codex plugin output")
		}
		return "", "", nil
	}
	if name == "" {
		name = DefaultPackageName
	}
	// Preserve the common npm-style safety constraints before host validation.
	if err := ValidatePackageName(name); err != nil {
		return "", "", err
	}
	if err := adapter.ValidateName(name); err != nil {
		return "", "", err
	}
	if version == "" {
		version = DefaultPackageVersion
	}
	if err := ValidatePackageVersion(version); err != nil {
		return "", "", err
	}
	return name, version, nil
}
