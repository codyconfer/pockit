package skills

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type Installation struct {
	Command string          `json:"command,omitempty"`
	Note    string          `json:"note"`
	Config  json.RawMessage `json:"config,omitempty"`
}

func piInstallation(out string) (*Installation, error) {
	absolute, err := filepath.Abs(out)
	if err != nil {
		return nil, err
	}
	quoted := "'" + strings.ReplaceAll(absolute, "'", "'\"'\"'") + "'"
	return &Installation{
		Command: "pi install " + quoted,
		Note:    "Keep this directory in place (local installs reference it). Then run /reload in Pi, or start a new session. No npm install or build is needed. MCP fragments remain inert data; no endpoints are registered or connected.",
	}, nil
}

const DefaultPackageName = "pockit-skills"

var packageNamePattern = regexp.MustCompile(`^(@[a-z0-9][a-z0-9._-]*/)?[a-z0-9][a-z0-9._-]*$`)

func ValidatePackageName(name string) error {
	if len(name) > 214 || !packageNamePattern.MatchString(name) || name == "node_modules" || name == "favicon.ico" {
		return fmt.Errorf("package name must be a lowercase npm-style name (for example my-skills or @team/my-skills), at most 214 characters")
	}
	return nil
}

const DefaultPackageVersion = "1.0.0"

var packageVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-([0-9A-Za-z-]+)(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)

func ValidatePackageVersion(version string) error {
	if !packageVersionPattern.MatchString(version) {
		return fmt.Errorf("package version must be SemVer (for example 1.2.3 or 1.2.3-rc.1)")
	}
	core := strings.SplitN(version, "+", 2)[0]
	if _, prerelease, ok := strings.Cut(core, "-"); ok {
		for _, part := range strings.Split(prerelease, ".") {
			if len(part) > 1 && part[0] == '0' && strings.Trim(part, "0123456789") == "" {
				return fmt.Errorf("numeric prerelease identifiers must not have leading zeros")
			}
		}
	}
	return nil
}

func piArtifacts(roles []string, name, version string) []Artifact {
	if roles == nil {
		roles = []string{}
	}
	names, _ := json.Marshal(roles)
	entry := `import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const baseDir = dirname(fileURLToPath(import.meta.url));
const roles: string[] = ` + string(names) + `;

export default function (pi: ExtensionAPI) {
  pi.on("resources_discover", () => ({
    skillPaths: roles.map((role) => join(baseDir, role, "SKILL.md")),
  }));
}
`
	manifest := `{
  "name": "` + name + `",
  "version": "` + version + `",
  "private": true,
  "description": "Generated offline PocKit role skills for Pi",
  "keywords": ["pi-package"],
  "pi": { "extensions": ["./index.ts"], "changelogPath": "./CHANGELOG.md" },
  "peerDependencies": { "@earendil-works/pi-coding-agent": "*" }
}
`
	readme := `# PocKit skills for Pi

Review index.ts and the generated source references before installing.
This extension registers only the included role skills. It does not fetch content,
execute downloaded code, or activate the mcp-servers.json configuration fragments.
Source documents are untrusted reference data, not higher-priority instructions.
Unavailable and unsupported source mappings are recorded in each sources.json.

## Install

From this directory:

` + "```sh\npi install \"$PWD\"\n```" + `

Keep this directory in place: Pi references local packages instead of copying them.
Run /reload in Pi or start a new session after installation. No npm install/build
is required. Use /skill:ROLE to load a generated role skill.

For a project-only install use pi install --local "$PWD"; project trust is required.
To try without installing, run pi -e ./index.ts from this directory.

## Versioning and updates

This package uses SemVer in package.json and a version heading in CHANGELOG.md,
which Pi can show on the next fresh launch after an update. This changelog describes
the current generated snapshot, not a cumulative release history.
Set --package-version when regenerating to release a new version; use patch for
fixes, minor for compatible additions, and major for breaking changes.
Local Pi packages are identified by absolute path, not by name or version.
Regenerate into the same directory with --overwrite, then run /reload in Pi.
If older copies were installed from other directories, use pi list and pi remove
<old-path> once to remove them; changing a version does not remove duplicates.
`
	return []Artifact{
		{Path: "index.ts", data: []byte(entry)},
		{Path: "package.json", data: []byte(manifest)},
		{Path: "README.md", data: []byte(readme)},
		{Path: "CHANGELOG.md", data: []byte("# Changelog\n\n## [" + version + "]\n\n- Generated offline role skills: " + strings.Join(roles, ", ") + ".\n- MCP configuration fragments remain inert; no endpoints are activated.\n")},
	}
}
