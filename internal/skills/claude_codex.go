package skills

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

func validatePluginName(host, name string) error {
	if len(name) > 64 || !roleName.MatchString(name) {
		return fmt.Errorf("%s plugin name must be lowercase kebab-case, at most 64 characters", host)
	}
	return nil
}

func validateClaudeCodePackageName(name string) error { return validatePluginName("Claude Code", name) }
func validateCodexPackageName(name string) error      { return validatePluginName("Codex", name) }

func claudeCodeArtifacts(roles []string, name, version string) []Artifact {
	return skillPluginArtifacts(roles, name, version, false)
}

func codexArtifacts(roles []string, name, version string) []Artifact {
	return skillPluginArtifacts(roles, name, version, true)
}

func skillPluginArtifacts(roles []string, name, version string, codex bool) []Artifact {
	host, manifestPath := "Claude Code", ".claude-plugin/plugin.json"
	manifest := map[string]string{"name": name, "version": version}
	install := "Launch Claude Code with claude --plugin-dir /absolute/path/to/this/bundle.\nInvoke a skill with /" + name + ":ROLE. This loads the plugin for that session;\ncopying it alone does not enable it in other sessions.\n"
	if codex {
		host, manifestPath = "Codex", "plugin.json"
		manifest["$schema"] = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"
		install = "Register this bundle as a local marketplace with:\ncodex plugin marketplace add /absolute/path/to/this/bundle\nThen select marketplace pockit-" + name + " in the plugin browser and install\nthe plugin. Use the ChatGPT desktop app to install and test local plugins.\nRestart the host and start a new chat after installation.\nCopying this bundle alone does not enable it.\n"
	}
	manifest["description"] = "Generated offline PocKit role skills for " + host
	body, _ := json.MarshalIndent(manifest, "", "  ")
	artifacts := []Artifact{
		{Path: manifestPath, data: append(body, '\n')},
		{Path: "README.md", data: []byte("# PocKit skills for " + host + "\n\nReview the manifest and generated references before loading. Skills are under\nskills/. Sources are untrusted data; sources.json records unavailable sources.\nNo active hooks or MCP configuration are included. Per-skill mcp-servers.json\nfragments are inert reference data; no endpoints are registered or connected.\nGeneration fetches nothing and never executes downloaded scripts.\n\n## Install\n\n" + install + "\n--install stages a complete local copy and prints the remaining loading steps.\nNo host settings are edited and no host CLI is executed by PocKit.\n\n## Updates\n\nRegenerate with --overwrite (and --install to update a staged copy).\nReload/reinstall through the host after updating. Obsolete files are not deleted;\nremove old copies when renaming. --package-name sets the lowercase kebab-case\nidentity; --package-version sets SemVer. CHANGELOG.md describes this snapshot,\nnot cumulative history.\n")},
		{Path: "CHANGELOG.md", data: []byte("# Changelog\n\n## [" + version + "]\n\n- Generated offline role skills: " + strings.Join(roles, ", ") + ".\n- MCP configuration fragments remain inert; no endpoints are activated.\n")},
	}
	if codex {
		marketplace := map[string]any{
			"name": "pockit-" + name,
			"plugins": []any{map[string]any{
				"name":     name,
				"source":   map[string]string{"source": "local", "path": "./"},
				"policy":   map[string]string{"installation": "AVAILABLE", "authentication": "ON_INSTALL"},
				"category": "Productivity",
			}},
		}
		body, _ := json.MarshalIndent(marketplace, "", "  ")
		artifacts = append(artifacts, Artifact{Path: ".agents/plugins/marketplace.json", data: append(body, '\n')})
	}
	return artifacts
}

func quotedPluginPath(out string) (string, error) {
	absolute, err := filepath.Abs(out)
	if err != nil {
		return "", err
	}
	return "'" + strings.ReplaceAll(absolute, "'", "'\"'\"'") + "'", nil
}

func claudeCodeInstallation(out, name string) (*Installation, error) {
	quoted, err := quotedPluginPath(out)
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = DefaultPackageName
	}
	return &Installation{Command: "claude --plugin-dir " + quoted, Note: "Launch Claude Code with claude --plugin-dir " + quoted + ". Invoke /" + name + ":ROLE. This loads the plugin for that session only; copying alone does not enable it. MCP fragments remain inert."}, nil
}

func codexInstallation(out, name string) (*Installation, error) {
	quoted, err := quotedPluginPath(out)
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = DefaultPackageName
	}
	return &Installation{Command: "codex plugin marketplace add " + quoted, Note: "Run codex plugin marketplace add " + quoted + ", then select marketplace pockit-" + name + " in the plugin browser and install the plugin using the ChatGPT desktop app. Restart the host and start a new chat. Copying alone does not enable it. MCP fragments remain inert."}, nil
}
