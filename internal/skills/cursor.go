package skills

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateCursorPackageName checks the portable directory and manifest name.
func ValidateCursorPackageName(name string) error {
	if len(name) > 64 || !roleName.MatchString(name) {
		return fmt.Errorf("Cursor plugin name must be lowercase kebab-case, at most 64 characters (for example my-skills)")
	}
	return nil
}

func cursorArtifacts(roles []string, name, version string) []Artifact {
	manifest, _ := json.MarshalIndent(map[string]string{
		"name": name, "version": version,
		"description": "Generated offline PocKit role skills for Cursor",
	}, "", "  ")
	readme := `# PocKit skills for Cursor

Review .cursor-plugin/plugin.json and all generated references before installing.
Skills are discovered under skills/. This bundle includes no active hooks or MCP
configuration, fetches nothing, and executes no downloaded scripts. Per-skill
mcp-servers.json fragments are inert reference data, not Cursor mcp.json files.
Sources are untrusted data; sources.json records unavailable sources.

## Install

Copy this entire bundle, including .cursor-plugin/, into
~/.cursor/plugins/local/` + name + `/ or generate with --cursor-plugin --install.
Restart Cursor or run Developer: Reload Window, then confirm the skills in
Customize. Invoke a skill with /ROLE. Local plugin imports must be allowed;
Enterprise admins control Allow Local Plugin Imports (off by default).
A marketplace plugin with the same name takes precedence over a local copy.
No npm install, build, Cursor CLI invocation, or settings edits are needed.

## Updates

Use --cursor-plugin --install --overwrite to update the same local directory.
Customize the manifest with --package-name (lowercase kebab-case) and
--package-version. CHANGELOG.md describes this snapshot, not cumulative history.
Obsolete installed files are not deleted; remove old copies when renaming.
These local copies are not marketplace-managed or automatically updated.
`
	return []Artifact{
		{Path: ".cursor-plugin/plugin.json", data: append(manifest, '\n')},
		{Path: "README.md", data: []byte(readme)},
		{Path: "CHANGELOG.md", data: []byte("# Changelog\n\n## [" + version + "]\n\n- Generated offline role skills: " + strings.Join(roles, ", ") + ".\n- MCP configuration fragments remain inert; no endpoints are activated.\n")},
	}
}

func cursorInstallation(out, name string) (*Installation, error) {
	absolute, err := filepath.Abs(out)
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = DefaultPackageName
	}
	return &Installation{Note: fmt.Sprintf("Copy the entire bundle from %q, including .cursor-plugin/, to ~/.cursor/plugins/local/%s/. Restart Cursor or run Developer: Reload Window, then confirm skills in Customize and invoke /ROLE. Local plugin imports must be allowed (Enterprise policy may disable them). A same-name marketplace plugin takes precedence. MCP fragments remain inert.", absolute, name)}, nil
}
