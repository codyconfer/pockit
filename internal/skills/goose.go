package skills

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

func validateGoosePackageName(name string) error {
	if err := ValidatePackageName(name); err != nil {
		return err
	}
	if strings.ContainsAny(name, "@/") {
		return fmt.Errorf("Goose plugin name must be unscoped (for example my-skills)")
	}
	return nil
}

func gooseArtifacts(roles []string, name, version string) []Artifact {
	manifest, _ := json.MarshalIndent(map[string]string{
		"name": name, "version": version,
		"description": "Generated offline PocKit role skills for Goose",
	}, "", "  ")
	readme := `# PocKit skills for Goose

Review plugin.json and the generated source references before installing.
This Open Plugins bundle provides only skills under skills/. It includes no hooks,
fetches no content, executes no downloaded scripts, and leaves mcp-servers.json
fragments inert. Sources are untrusted reference data. Each role's sources.json
records unavailable and unsupported sources.

## Install

Copy this entire directory into ~/.agents/plugins/` + name + `/ for user-wide
use, or <project>/.agents/plugins/` + name + `/ for one project. Keep plugin.json
at the plugin root and preserve the skills/ directory and all supporting files.
Use a Goose version with Open Plugins support and start a new session after
installation or updates. Load a role by its namespaced name: ` + name + `:ROLE.
No npm install, build, or MCP server is needed.

Alternatively, generate with --goose-plugin --install to copy the bundle into
~/.agents/plugins/NAME/ for the current user. Existing installations require
--overwrite. PocKit does not edit Goose settings. Goose's plugin install command
is for git repositories; local copies are not managed by it.

## Updates

Regenerate with --overwrite and copy the updated bundle to the same plugin
location, or use --goose-plugin --install --overwrite. Set --package-name (unscoped) and --package-version to customize the
manifest. CHANGELOG.md describes the current snapshot, not cumulative history.
Manually copied plugins are not managed by goose plugin update or auto-update.
If renaming or moving a plugin, remove its old installed copy to avoid duplicates.
`
	return []Artifact{
		{Path: "plugin.json", data: append(manifest, '\n')},
		{Path: "README.md", data: []byte(readme)},
		{Path: "CHANGELOG.md", data: []byte("# Changelog\n\n## [" + version + "]\n\n- Generated offline role skills: " + strings.Join(roles, ", ") + ".\n- MCP configuration fragments remain inert; no endpoints are activated.\n")},
	}
}

func gooseInstallation(out, name string) (*Installation, error) {
	absolute, err := filepath.Abs(out)
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = DefaultPackageName
	}
	return &Installation{
		Note: fmt.Sprintf("Copy the entire bundle from %q to ~/.agents/plugins/%s/ (user) or <project>/.agents/plugins/%s/ (project), then start a new Goose session. Load skills as %s:ROLE. Requires Open Plugins support. Manually copied plugins do not use Goose auto-update. MCP fragments remain inert.", absolute, name, name, name),
	}, nil
}
