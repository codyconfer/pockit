# PocKit

PocKit turns a source catalog into compact, offline role-based skills, Pi packages,
and plugins for OpenCode, Goose, Cursor, Claude Code, or Codex. It runs as a single
Go binary with embedded defaults. You can generate packages without Node.js,
Python, or an external service; you need an agent host only to use them.

```text
Catalog + roles → validate / plan → explicit fetch → private cache
                                                        ↓
                                  offline skills / host-specific packages and plugins
```

## Quick start

To build PocKit, you need Go 1.26.5+.

```sh
make build
./pockit sources validate
./pockit sources plan --role as-reviewer --json
# Review the plan before authorizing remote collection.
./pockit sources fetch --role as-reviewer
./pockit skills generate --role as-reviewer --out /private/skills
```

- To build without Make, run `go build -o pockit ./cmd/pockit`.
- Run `make install` to install the binary (default `~/.local/bin/pockit`). Use
  `BINDIR=/your/bin` to choose another directory, and add it to `PATH` if needed.
- Omit `--role` to include all roles. Put flags after the command.

## Commands

| Command | Purpose |
| --- | --- |
| `config init` | Install editable sources and roles in the user config directory |
| `config schema [--out FILE]` | Export the bundled source-catalog JSON schema |
| `input --sources FILE --roles FILE --out DIR` | Generate an ad-hoc offline bundle; ignore installed config |
| `sources validate` | Validate the whole catalog and inheritance graph offline |
| `sources plan --json` | Inspect resolved sources, mappings, and cache state |
| `sources fetch` | Explicitly fetch supported documents and scripts |
| `skills generate --out DIR` | Build role skills from cached content, offline |

| Common option | Effect |
| --- | --- |
| `--role NAME` | Select a role and inherited sources |
| `--dry-run --json` | Preview generation without writing |
| `--check --out DIR --json` | Check output against cached inputs without writing |
| `--overwrite` | Permit artifact replacement |
| `--cache DIR` | Use a private cache outside Git checkouts and Go modules |
| `--sources FILE`, `--roles FILE`, `--schema FILE` | Override inputs (`--catalog` / `--inheritance` are aliases for the first two) |

## Configuration

```sh
./pockit config init
./pockit config schema --out sources.schema.json
./pockit input --sources ./sources.json --roles ./roles.json --out /private/one-off
```

For normal commands, PocKit chooses each input independently: an explicit file
first, then installed user config, then the embedded default. The default schema
is always embedded. On Linux, user config lives in `$XDG_CONFIG_HOME/pockit` or
`~/.config/pockit`. If an installed file is invalid, PocKit reports an error
instead of falling back.

To replace existing config files, pass `--overwrite`. PocKit never replaces
symlinks or directories. `config init --overwrite` restores the bundled defaults.
`input` requires both files and accepts generation flags, but never fetches
sources. Populate its cache first with `sources fetch --sources FILE --roles FILE`.
See [configuration details](docs/reference.md#configuration-and-validation).

## Use with Pi

```sh
./pockit skills generate --role as-reviewer --out /private/pockit-pi --pi-extension
# Review the generated files before installing.
pi install /private/pockit-pi
```

Reload Pi resources with `/reload` (or start a new session), then use
`/skill:as-reviewer`. Keep the output directory: Pi references it in place.

| Task | Generation options |
| --- | --- |
| Generate only | `--pi-extension` |
| Generate and install | `--pi-extension --install` |
| Update in place | `--pi-extension --install --overwrite --package-version 1.1.0` |
| Name the package | `--pi-extension --package-name my-skills` (or `@team/my-skills`) |

Without `--out`, PocKit writes Pi output to `~/.local/share/pockit/pi-package`
(or absolute `$XDG_DATA_HOME/pockit/pi-package`), reusing the same directory.
Packages default to the name `pockit-skills` and version `1.0.0`, and include a
changelog. Bumping the version does not remove duplicate installations. Check
`pi list`, remove old paths with `pi remove <old-path>`, then reload. See [package versioning](docs/reference.md#pi-package-versions).

## Use with OpenCode

```sh
./pockit skills generate --role as-reviewer --opencode-plugin
# After reviewing the output, regenerate and install for the current user:
./pockit skills generate --role as-reviewer --opencode-plugin --install --overwrite
```

Review the generated files, then merge the printed `installation.config` snippet
into `opencode.json` for the project or `~/.config/opencode/opencode.json` for
global use. Append to any existing `plugin` array. Restart OpenCode and load the
role with its native skill tool. Keep the entire output directory in place.

Without `--out`, PocKit reuses `~/.local/share/pockit/opencode-plugin` (or absolute
`$XDG_DATA_HOME/pockit/opencode-plugin`). Pass `--overwrite` for updates. Package
name and version flags work as they do for Pi, and no npm install or build is
needed. `--install` adds the plugin URL to the user config at
`$XDG_CONFIG_HOME/opencode/opencode.json` (or `~/.config/opencode/opencode.json`
on Linux). It keeps unrelated settings and skips duplicate URLs. Merge existing
JSONC configs manually. You can select only one host output format at a time.
See [OpenCode output](docs/reference.md#opencode-plugin-output).

## Use with Goose

```sh
./pockit skills generate --role as-reviewer --goose-plugin --out /private/pockit-goose
# Review plugin.json and the generated references before copying.
mkdir -p ~/.agents/plugins/pockit-skills
cp -R /private/pockit-goose/. ~/.agents/plugins/pockit-skills/
# Or generate and install for the current user automatically:
./pockit skills generate --role as-reviewer --goose-plugin --install
```

Use a Goose version with Open Plugins support. Start a new session and ask to
load `pockit-skills:as-reviewer`. To use the plugin only in one project, copy it
to `<project>/.agents/plugins/pockit-skills/` instead. Keep the whole bundle,
including `plugin.json` and `skills/`.

Without `--out`, PocKit reuses `~/.local/share/pockit/goose-plugin` (or absolute
`$XDG_DATA_HOME/pockit/goose-plugin`). The default name and version are
`pockit-skills` and `1.0.0`; Goose names must be unscoped. `--install` copies the
entire bundle to `~/.agents/plugins/NAME/`. Pass `--overwrite` when updating both
the generated output and the installed copy. PocKit leaves obsolete files in
place, and neither `goose plugin update` nor auto-update manages these local
copies. For any host, `--install` cannot be combined with `--dry-run` or `--check`.
See [Goose output](docs/reference.md#goose-plugin-output).

## Use with Cursor

```sh
./pockit skills generate --role as-reviewer --cursor-plugin
# Review the generated files, then generate and install a local copy:
./pockit skills generate --role as-reviewer --cursor-plugin --install --overwrite
```

The bundle contains `.cursor-plugin/plugin.json` and `skills/ROLE/`, including
all offline references. Without `--out`, output reuses
`~/.local/share/pockit/cursor-plugin` (or absolute
`$XDG_DATA_HOME/pockit/cursor-plugin`). `--install` copies the entire bundle,
including the hidden manifest directory, to `~/.cursor/plugins/local/NAME/`.
Pass `--overwrite` to update an existing bundle; PocKit does not delete obsolete
files. Names default to `pockit-skills` and must be lowercase kebab-case, at most
64 characters long. `--package-version` defaults to `1.0.0`.

Restart Cursor or run **Developer: Reload Window**, check **Customize**, then
invoke `/as-reviewer`. Enterprise admins must allow local plugin imports; this
setting is off by default. A marketplace plugin with the same name takes
precedence over the local copy. The marketplace does not manage these copies.
PocKit leaves settings unchanged and MCP fragments inactive. See
[Cursor output](docs/reference.md#cursor-plugin-output).

## Use with Claude Code or Codex

```sh
./pockit skills generate --role as-reviewer --claude-code-plugin --out /private/pockit-claude
# Review the generated files, then load for this session:
claude --plugin-dir /private/pockit-claude
# Invoke /pockit-skills:as-reviewer

./pockit skills generate --role as-reviewer --codex-plugin --out /private/pockit-codex
# Review the generated files, then register the local marketplace:
codex plugin marketplace add /private/pockit-codex
```

Claude Code bundles contain `.claude-plugin/plugin.json` and `skills/ROLE/`.
Codex bundles contain a portable root `plugin.json`, `skills/ROLE/`, and a local
marketplace catalog at `.agents/plugins/marketplace.json`. In the plugin browser,
select marketplace `pockit-pockit-skills` and install using the ChatGPT desktop
app. Restart the app and start a new chat. Host availability and policy may vary.

Without `--out`, output reuses `~/.local/share/pockit/claude-code-plugin` or
`~/.local/share/pockit/codex-plugin` (under absolute `$XDG_DATA_HOME` if set).
`--install` places complete local copies under `~/.claude/plugins/local/NAME/` or
`~/.codex/plugins/local/NAME/` and prints instructions for loading or registering
them. These are PocKit staging directories; neither host discovers plugins there
automatically. PocKit does not invoke either CLI or edit host settings. Follow
the printed steps to enable the plugin.

Names default to `pockit-skills` and must be lowercase kebab-case, with a
64-character limit. Versions default to `1.0.0`. Pass `--overwrite` for updates;
PocKit does not delete obsolete files. Reload or reinstall through the host after
updating. MCP fragments stay inactive, and PocKit generates no active hooks. See
[adapter details](docs/reference.md#claude-code-and-codex-plugin-output).

## Bundle upstream documentation

You can also bundle Pi, OpenCode, Goose, and Cursor docs as offline references.
The [example catalog](examples/agent-docs/sources.json) lists individual Markdown
and MDX files, plus Cursor's published Markdown pages. PocKit does not crawl
repositories or render MDX. Validate the catalog, review the plan, fetch the
sources explicitly, then generate with the host flag you need. The nightly
workflow refreshes docs for all four hosts and publishes checked Pi, OpenCode,
Goose, and Cursor bundles. See the [workflow](docs/reference.md#repository-documentation-bundles).

## Single AGENT.md output

```sh
./pockit skills generate --role as-reviewer --agent-md --out /private/agent-bundle
```

`--agent-md` writes a single file, `/private/agent-bundle/AGENT.md`, with selected
roles, cached reference text, script snapshots, provenance, source status, and
inert MCP suggestions. Omit `--role` to combine all roles. Binary content is
base64-encoded. Included content needs no companion files or fetches, and
upstream links stay unchanged. Missing content produces a partial result.

You can use this mode with `input` too. The `--dry-run`, `--check --out DIR`, and
`--overwrite` flags work as they do for other outputs. Without `--out`, PocKit
uses a private temporary directory. You cannot combine this flag with any package
output flag, including `--claude-code-plugin` and `--codex-plugin`. Package options
and `--install` do not apply.

## Output and safety

Generated skills link to complete local source files, preserving their original
bytes. Load only the references you need for the task. Bundles also include
provenance, supported scripts, and inert MCP connection suggestions; they do not
open live connections.

| Signal | Meaning |
| --- | --- |
| Default terminal output | Inline ViewKit panels for progress, results, warnings, and installation instructions; no full-screen UI |
| Progress reaches 100% | Processing finished; some sources may still have failed |
| `--json` | Full reports for inspection and automation, without panels |
| `--plain`, `TERM=dumb`, or redirection | Plain text with a final progress bar; no panels or cursor controls |
| `NO_COLOR` | Keep terminal panels but omit color |
| Exit 0 / 2 / 3 / 4 | Success / invalid input / unsafe or refused output / partial result or operational failure |

A partial run may still leave useful cache entries or artifacts. Check its
warnings or JSON report.

- Only `sources fetch` accesses remote content, and only at supported public HTTPS
  destinations. It does not crawl, connect to live services, or execute downloaded scripts.
- GitHub credentials: `GITHUB_TOKEN` → `GH_TOKEN` → existing `gh auth login`.
  Use least privilege; never put tokens in the catalog.
- Cache defaults to the OS user cache directory under `pockit` (typically
  `~/.cache/pockit` on Linux). Cache and generated private content are not
  encrypted; keep them private and out of version control.
- Inherited references do not grant permissions or authority.

## Development and reference

`make check` runs formatting, vet, tests, and race checks without live credentials.
Tests derive their default-role expectations from the embedded catalog and
inheritance graph instead of assuming a fixed role count.

`make test-integration` builds an amd64 Docker image with Pi, OpenCode, Goose,
and Cursor CLI, then tests package installation and upgrades without network
access or API keys. The tests require Pi, OpenCode, and Goose to discover the
generated skills through their real loaders. For Cursor, they check the installed
plugin files and run a CLI smoke test; they do not test desktop activation. See [integration tests](integration/README.md) for versions,
coverage, and limitations. GitHub Actions runs the same container on pull requests
and pushes to `main`. `make help` lists all build targets.

| Document | Contents |
| --- | --- |
| [Reference](docs/reference.md) | Flags, source support, network policy, cache, artifacts, Pi/OpenCode/Goose installation |
| [JSON compatibility](docs/reference.md#json-compatibility-changes) | Removed serving interface and local-bundle mappings |
| [Output measurements](docs/skill-output-size.md) | Historical token estimates and source-preservation checks |
| [Verification](docs/verification.md) | Tested safety boundaries and known limitations |

## License

PocKit is licensed under the [GNU General Public License v3.0](LICENSE).
