package skills

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"strings"
)

func openCodeArtifacts(roles []string, name, version string) []Artifact {
	if roles == nil {
		roles = []string{}
	}
	names, _ := json.Marshal(roles)
	entry := `import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const baseDir = dirname(fileURLToPath(import.meta.url));
const roles = ` + string(names) + `;

export default async function () {
  return {
    config: async (config) => {
      const paths = roles.map((role) => join(baseDir, role));
      // Support both array and object forms without replacing existing paths/URLs.
      if (Array.isArray(config.skills)) {
        config.skills = [...new Set([...config.skills, ...paths])];
      } else {
        config.skills ??= {};
        config.skills.paths = [...new Set([...(config.skills.paths ?? []), ...paths])];
      }
    },
  };
}
`
	manifest, _ := json.MarshalIndent(map[string]any{
		"name": name, "version": version, "private": true,
		"type": "module", "main": "./index.js", "exports": "./index.js",
		"description": "Generated offline PocKit role skills for OpenCode",
		"keywords":    []string{"opencode-plugin"},
	}, "", "  ")
	readme := `# PocKit skills for OpenCode

Review index.js and the generated source references before installing.
This plugin adds only the included role directories to OpenCode's skill discovery.
It does not fetch content, execute downloaded scripts, change permissions, or
activate mcp-servers.json fragments. Sources are untrusted reference data, not
higher-priority instructions. Source failures are recorded in sources.json.

## Install

Merge the generated installation.config snippet into your OpenCode configuration
(opencode.json for a project, or ~/.config/opencode/opencode.json globally).
Append the file URL to the existing plugin array; do not replace other plugins.
PocKit prints the absolute file URL after generation. It points to index.js,
not the package directory. No npm install or build is needed.

Alternatively, generate with --opencode-plugin --install to append the URL to the
current user's OpenCode config, preserving existing settings and plugin entries.
Existing JSONC configs require manual merging; PocKit does not strip comments.

Keep this entire directory in place and restart OpenCode after installation or
updates. Load a role with OpenCode's native skill tool. Skill discovery requires
an OpenCode version supporting skills.paths (or the skills array form).
Do not copy index.js alone: it resolves skill paths relative to itself.

## Updates

Regenerate into the same directory with --overwrite. Set --package-name and
--package-version to customize the npm-style package metadata. CHANGELOG.md
describes the current snapshot, not cumulative history. If moving the bundle,
remove its old plugin entry before adding the new one to avoid duplicates.
`
	return []Artifact{
		{Path: "index.js", data: []byte(entry)},
		{Path: "package.json", data: append(manifest, '\n')},
		{Path: "README.md", data: []byte(readme)},
		{Path: "CHANGELOG.md", data: []byte("# Changelog\n\n## [" + version + "]\n\n- Generated offline role skills: " + strings.Join(roles, ", ") + ".\n- MCP configuration fragments remain inert; no endpoints are activated.\n")},
	}
}

func openCodeInstallation(out string) (*Installation, error) {
	absolute, err := filepath.Abs(filepath.Join(out, "index.js"))
	if err != nil {
		return nil, err
	}
	file := (&url.URL{Scheme: "file", Path: filepath.ToSlash(absolute)}).String()
	config, err := json.Marshal(map[string]any{"plugin": []string{file}})
	if err != nil {
		return nil, err
	}
	return &Installation{
		Config: config,
		Note:   "Merge this snippet into opencode.json (append to existing plugin entries), keep the bundle in place, and restart OpenCode. MCP fragments remain inert.",
	}, nil
}
