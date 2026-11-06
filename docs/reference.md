# PocKit reference

[Quick start](../README.md) · [Safety evidence](verification.md) ·
[Historical output measurements](skill-output-size.md)

```text
Catalog + inheritance → validate → resolve plan → explicit fetch → private cache
                                      └──────────────────────────────┤
                                                        offline generation
                                                                 ↓
                                      role skills / Pi package / OpenCode or Goose plugin
```

## Build and development

You need Go 1.26.5+ to build PocKit. The binary includes the catalog, Draft
2020-12 schema, and inheritance graph, so you can run it outside the checkout.
It needs no Node.js, Python, agent installation, or external service.

```sh
go mod download
go build -trimpath -buildvcs=false -ldflags=-buildid= -o /tmp/pockit ./cmd/pockit
cd /tmp
./pockit sources validate --json
./pockit sources plan --role as-reviewer --json
```

| Make target | Action |
| --- | --- |
| `make` / `make build` | Build `./pockit` with the flags above |
| `make install` | Native binary to `~/.local/bin`; override `BINDIR` or `PREFIX` |
| `make deps` | Download dependencies |
| `make test` / `make test-race` | Tests / race tests (race detector requires CGO) |
| `make vet` / `make fmt` | Static checks / format Go sources |
| `make check` | Check formatting, vet, tests, and race tests |
| `make clean` / `make help` | Remove binary / list targets |

`go.mod` pins dependencies, including jsonschema/v6 v6.0.3 and yaml.v3 v3.0.1.
The bundled `data/sources.json` and `data/sources.schema.json` preserve the supplied
inputs. They contain URLs and annotations, with no fetched private documents or
credentials.

## Commands and flags

Flags follow the command. Omit `--role` to select all roles.

| Command | Action |
| --- | --- |
| `config init` | Install editable bundled sources and roles |
| `config schema [--out FILE]` | Export source-catalog schema |
| `input --sources FILE --roles FILE` | Generate an ad-hoc offline bundle |
| `sources validate` | Validate the whole input, regardless of selected role |
| `sources plan --json` | Inspect resolved mappings and cache state |
| `sources fetch` | Explicitly authorize supported remote collection |
| `skills generate` | Generate from cached content only |

| Flags | Scope and behavior |
| --- | --- |
| `--catalog FILE` / `--sources FILE` | Catalog override (aliases) |
| `--inheritance FILE` / `--roles FILE` | Role graph override (aliases) |
| `--schema FILE`, `--role NAME`, `--cache DIR` | Schema, selected role, private cache |
| `--json`, `--plain` | Full JSON report / plain text without panels or cursor controls |
| `--refresh` | Fetch: recollect successful cached URLs |
| `--concurrency N` | Fetch: 1 to 8 workers, default 8 |
| `--timeout DURATION` | Fetch: positive per-request timeout, default `30s` |
| `--max-bytes N` | Fetch: positive per-response limit, default `8388608` (8 MiB) |
| `--out DIR` | Generation destination; required for `--check` |
| `--dry-run` | Preview generation without writes; may return 4 for incomplete output |
| `--overwrite` | Permit replacement of regular generated artifacts |
| `--check` | Offline comparison without writes; incompatible with `--dry-run` or `--overwrite` |
| `--pi-extension` | Include a Pi package; mutually exclusive with other host outputs and `--agent-md` |
| `--opencode-plugin` | Include an OpenCode plugin and manual configuration snippet; mutually exclusive with other host outputs and `--agent-md` |
| `--goose-plugin` | Include a Goose Open Plugins bundle and manual copy instructions; mutually exclusive with other host outputs and `--agent-md` |
| `--cursor-plugin` | Include a Cursor manifest and nested skills; mutually exclusive with other host outputs and `--agent-md` |
| `--claude-code-plugin` | Include a Claude Code manifest and nested skills; mutually exclusive with other host outputs and `--agent-md` |
| `--codex-plugin` | Include a portable Codex manifest, nested skills, and local marketplace; mutually exclusive with other host outputs and `--agent-md` |
| `--agent-md` | Write only `AGENT.md` in `--out DIR`, combining all selected roles and cached content; no companion files |
| `--install` | Install the generated package for the current user (Claude Code/Codex stage a local copy with remaining activation instructions); requires a host output flag, forbids `--dry-run` / `--check` |
| `--package-name NAME`, `--package-version SEMVER` | Package identity; require a package output flag |

Fetch limits apply per request; workers can run concurrently. You can build and
test PocKit without fetching production sources.

`--agent-md` works with `skills generate` and `input`. Without `--out`, PocKit uses
a private temporary directory. The file contains fenced text snapshots, binary
content encoded as base64, role-specific source manifests, provenance indexes,
and inert MCP fragments. Metadata paths refer to sections within the file, not
to files on disk. Original source links stay unchanged; resolve relative links
against the embedded original URLs. Script snapshots are read-only references
with explicit checkout, dependency, and authorization prerequisites.

Missing or unsupported sources stay visible and cause exit 4. The `--dry-run`,
`--overwrite`, and `--check` flags work as usual; checks compare AGENT.md directly,
without a companion state file. Package identity flags and `--install` are not
supported in this mode. PocKit leaves unrelated files in place.

### Configuration and validation

For normal commands, PocKit chooses the catalog and roles independently:

```text
Explicit input flag → installed user config → embedded default (only if absent)
Schema flag         → embedded schema
```

| Operation | Contract |
| --- | --- |
| Installed config | OS user config directory under `pockit`; Linux: `$XDG_CONFIG_HOME/pockit` or `~/.config/pockit` |
| Invalid/unreadable installed file | Error, never silent fallback |
| `config init` | Create private `sources.json` and `roles.json`; `--json` reports paths |
| `config init --overwrite` | Reset both files to bundled defaults |
| `config schema` | Raw JSON stdout, or `--out FILE`; works even with invalid installed config |
| Config file writes | Refuse existing files without `--overwrite`; refuse symlinks/nonregular destinations; atomic per file, not across both files |
| `input --sources FILE --roles FILE` | Require both files, ignore installed config, accept all generation flags plus optional schema; no config edits or fetches |

Before ad-hoc generation, fetch the sources into the cache you plan to use:

```sh
pockit sources fetch --sources ./sources.json --roles ./roles.json --cache /private/cache
pockit input --sources ./sources.json --roles ./roles.json --cache /private/cache --out /private/adhoc
```

Custom roles normally need a matching graph; a role with no parents uses
`{"my-role": []}`. The exported schema describes sources, not inheritance.

Validation rejects external schema references without making network requests.
It also rejects unknown roles or parents, cycles, duplicate parents, malformed
formats, and duplicate nonempty URLs within a role. Empty URLs and extensible
kinds stay visible even when PocKit cannot fetch them.

### Exit statuses and writes

| Code | Meaning |
| --- | --- |
| 0 | Complete success or help |
| 2 | Invalid command, input, options, or role |
| 3 | Refused overwrite or unsafe output path |
| 4 | Partial fetch/generation, stale `--check`, cancellation, installation or I/O failure |

A run that exits with code 4 may still leave useful artifacts or successful cache
entries. Generation never fetches missing cache content. To replace existing
artifacts, pass `--overwrite`. Each file write is atomic; updating the directory
tree is not a single transaction. If you cancel, completed artifacts may remain.
PocKit does not delete unrelated or obsolete files.

## Roles, provenance, and dispositions

`data/roles.json` maps each role to its parents. In the bundled graph,
`as-grafana-engineer` has no parents and `as-reviewer` inherits from it. Custom
inputs can define other roles. Inheritance shares references, but grants no
credentials, permissions, or operational authority. It also does not tell you
whether annotations are still current.

PocKit groups identical URLs deterministically and keeps every direct and
inherited context: the selected role, origin/index, graph path, and annotations,
including conflicts. Missing URLs get distinct origin/index identities. IDs are
full SHA-256 identity hashes. Plans keep mappings and cache state for every
context, including unknown metadata fields and explicit nulls.

| Kind | Action |
| --- | --- |
| `gh_file_md`, `markdown` | Fetch original Markdown; package a reference |
| `html`, `docs_grafana`, `text`, `json` | Fetch one document; package locally |
| `gh_script` | Fetch and package original script; never execute |
| `mcp_grafana`, `mcp_endpoint` | Inert connection fragment; never contact endpoint |
| `gh_repository`, `gh_directory`, `gh_issue`, `gh_issues`, `gh_labels`, `gh_project`, `gh_pull_request`, `gh_workflow` | Pointer only; no cloning, enumeration, execution, or fetching |
| `slack`, `docs_google`, `docs_google_sheet`, `grafana_dashboard`, `grafana_irm`, `argo_rollout` | Pointer only; no live or fabricated data |
| Empty URL | Missing; incomplete disposition |
| Unknown kind | Unsupported; no implied execution or capability |

A shared URL can have several actions and annotations. PocKit keeps all contexts
even when they share source bytes. Pointers and endpoint fragments describe the
sources without opening live connections.

## Authentication, network policy, and private cache

### Credentials and fetching

PocKit checks `GITHUB_TOKEN` first, then `GH_TOKEN`, then an existing GitHub CLI
login. Use a least-privilege read token. `gh` is optional with a token environment
variable; otherwise authenticate with `gh auth login`.

| Boundary | Policy |
| --- | --- |
| CLI credential helper | `gh auth token --hostname github.com`, once, noninteractive, five-second timeout; only for explicit uncached GitHub fetches; failure falls back to unauthenticated requests |
| Secret handling | No credential flag; never put tokens in command lines, URLs, catalog context, or committed files. Reject URL userinfo and common credential query parameters before diagnostics |
| GitHub blob URLs | Translate `https://github.com/OWNER/REPO/blob/REF/PATH` Markdown/scripts to contents API raw media; only `api.github.com` receives tokens |
| GitHub refs | One URL segment; slash-containing branches need an unambiguous commit SHA or directly readable HTTPS Markdown URL |
| Other GitHub shapes | Refuse unsupported shapes and rendered Markdown HTML |
| Network | HTTPS port 443 only; reject localhost and private/link-local/reserved/transition IP ranges; validate all DNS answers and dial a validated IP without re-resolving |
| Redirects | At most five, same-origin only; revalidate every target |
| Proxies | Ignore environment HTTP proxies |
| Media | Accept text, JSON, GitHub raw media; scripts also allow shell/JavaScript media; bounded body reads |
| Failures | 401/403, non-200, timeout, oversize/binary, policy refusal, detected login page, or cache failure never count as content success |

Login detection uses heuristics and cannot prove access. Keep a host egress
policy in place alongside PocKit's network protections. PocKit treats source
files as data: it does not execute fetched code or follow source `SKILL.md`
instructions.

PocKit attempts each URL once per collection, even if it fails. Roles share
successful cache entries. Reports include outcomes, identities, and provenance,
but no tokens, authorization headers, or remote error bodies. A failed refresh
is still reported as a failure; earlier content-addressed bytes may remain until
you remove them manually.

### Cache storage

| Property | Contract |
| --- | --- |
| Default | `os.UserCacheDir()/pockit`; Linux: `$XDG_CACHE_HOME/pockit` or `$HOME/.cache/pockit` |
| Override | `--cache DIR`; paths inside Git checkouts or Go modules are refused |
| Permissions | Directories 0700, new files 0600; existing roots must already be private |
| Integrity | Verify content identity on reads and content-addressed reuse; refuse corruption |
| Filesystem safety | Reject traversal/symlink escapes; publish atomically |
| Privacy | Cache and generated content are unencrypted; use trusted private storage and never commit private bundles |
| Removal | Inspect the chosen path, then manually remove that `pockit` cache directory |

Digest checks detect corruption, not malicious replacement by the same OS user.

## Claude Code and Codex plugin output

Both adapters use the shared offline builder for references, scripts, provenance,
dry runs, checks and fingerprints, overwrite refusal, and safe copying. Each role
lives under `skills/ROLE/`. MCP fragments stay inactive, and the bundle has no
root MCP configuration or hooks. Treat its sources as untrusted references.

| Adapter | Flag | Manifest | Additional metadata |
| --- | --- | --- | --- |
| Claude Code | `--claude-code-plugin` | `.claude-plugin/plugin.json` | README and snapshot changelog |
| Codex | `--codex-plugin` | Root `plugin.json` with Agent Plugins 1.0.0 schema | README, snapshot changelog, `.agents/plugins/marketplace.json` |

Names default to `pockit-skills` and must be lowercase kebab-case, with a
64-character limit and no npm scopes. Versions default to `1.0.0` and must follow
SemVer. Without `--out`, PocKit writes to the user data directory under
`pockit/claude-code-plugin` or `pockit/codex-plugin`. Choose only one package output
flag or `--agent-md` per run. Both `skills generate` and `input` accept these flags.

For Claude Code, review the bundle, then run
`claude --plugin-dir /absolute/path/to/bundle` and invoke `/PACKAGE:ROLE`. The
plugin is loaded for that session only.
For Codex, run `codex plugin marketplace add /absolute/path/to/bundle`, then use
the plugin browser to install from marketplace `pockit-NAME` (for the default
name, `pockit-pockit-skills`). Use the ChatGPT desktop app for local installation
and testing, then restart and start a new chat. The bundled marketplace points
at `./` relative to its root and contains only this plugin.

`--install` copies the whole bundle to `~/.claude/plugins/local/NAME/` or
`~/.codex/plugins/local/NAME/`. Neither host discovers plugins in these staging
directories automatically. PocKit prints instructions for the staged copy, but
does not run host commands, edit configuration, or enable plugins. Review and
carry out those steps yourself. `installation.command` is the command you can
run to launch or register the plugin; PocKit does not execute it. The JSON report
describes the original generated output, and stderr gives the staged path after
copying.

Pass `--overwrite` to update both the generated output and the staged copy, then
reload or reinstall in the host. PocKit does not delete obsolete files, so remove
old copies when renaming. As with the other local-copy adapters, it refuses
symlinks, nonregular files, and overlapping directories. You cannot combine
`--install` with `--dry-run` or `--check`. The host integration container does not
test real Claude Code or Codex activation.

Formats and manual loading steps follow the
[Claude Code manifest reference](https://code.claude.com/docs/en/plugins-reference)
and [OpenAI plugin packaging guide](https://developers.openai.com/plugins/build/plugins).

## Pi extension output

`--pi-extension` adds `package.json`, `index.ts`, `CHANGELOG.md`, and an installation
`README.md`. The extension registers selected skills through Pi resource
discovery without activating endpoints or executing downloaded code. You need
Pi to use the package, but PocKit itself does not need Node.js.

```sh
pockit skills generate --role as-reviewer --out /private/pockit-pi --pi-extension
# Review extension and private reference data before installation.
pi install /private/pockit-pi
```

Reload resources with `/reload` or start a session, then use `/skill:as-reviewer`.
No npm install or build is needed. Keep the directory because Pi references it
in place. For a project-only installation, run
`pi install --local /absolute/path/to/package`; this requires project trust.

| Mode | Destination / behavior |
| --- | --- |
| Pi, no `--out` | Stable `~/.local/share/pockit/pi-package`, or absolute `$XDG_DATA_HOME/pockit/pi-package` |
| Standalone, no `--out` | New private system temp directory; path on stderr; not automatically removed |
| Explicit `--out DIR` | Overrides either default |
| Repeated generation | Requires `--overwrite`; the registered list is replaced with the selected roles |
| `--install` | Requires Pi on `PATH`; runs `pi install` with absolute output path after all artifacts are written |
| Install failure | Exit 4; generated files remain; Pi stdout/stderr go to stderr to preserve JSON stdout |

```sh
# Generate and install at the stable path.
pockit skills generate --role as-reviewer --pi-extension --install
# Update the same package.
pockit skills generate --role as-reviewer --pi-extension --install --overwrite --package-version 1.1.0
```

PocKit runs installation only when you pass `--install`. If all package files
were written, you can still install the package when some mappings are unavailable
(exit 4). Its manifests record those limitations. Dry runs, refusals, and
interrupted writes do not report the package as ready to install. After successful
writes, JSON includes `installation.command` and `installation.note`, and stderr
shows the instructions.

### Pi package versions

| Option / field | Contract |
| --- | --- |
| `--package-name NAME` | Default `pockit-skills`; lowercase npm-style or scoped name, maximum 214 characters; does not rename skills or change output path |
| `--package-version SEMVER` | Default `1.0.0`; SemVer 2.0.0 without leading `v`, e.g. `1.2.3`, `2.0.0-rc.1`, `1.2.3+build.2` |
| Version choice | Patch for fixes, minor for compatible additions, major for breaking changes; never auto-bumped |
| `--check` | Supply the same version to check a released snapshot |
| Changelog | `pi.changelogPath`, `## [VERSION]` headings; current snapshot only, not cumulative history |
| Pi notification | May show a new entry on a fresh launch after version increase; first launch only records the version |
| Fingerprint | Includes version/changelog bytes; report and bundle-state schema versions remain `1` |

Pi identifies local packages by absolute path. Changing a name or version leaves
duplicate installations in place, so update the same directory and choose distinct
output paths for separate packages. To migrate, check `pi list`, remove redundant
entries with `pi remove <old-path>` (`--local` for project entries), install the
stable package, and reload. PocKit does not remove installations or edit Pi settings.

## OpenCode plugin output

`--opencode-plugin` adds `package.json`, dependency-free ESM `index.js`,
`CHANGELOG.md`, and installation `README.md`. Its `config` hook adds only the
selected role directories to skill discovery. It keeps existing paths and URLs
and skips duplicate paths. Both `skills.paths` and the newer `skills` array form
are supported. Use an OpenCode version that supports additional skill paths.
The plugin does not change permissions, activate MCP fragments, fetch sources,
or execute bundled scripts. No npm install or build is required.

```sh
pockit skills generate --role as-reviewer --opencode-plugin --out /private/pockit-opencode
# Review the package, then merge the printed config into your OpenCode config.
```

After a successful write, JSON reports `installation.config`, for example:

```json
{"plugin":["file:///private/pockit-opencode/index.js"]}
```

Append that URL to existing `plugin` entries in project `opencode.json` or global
`~/.config/opencode/opencode.json`; do not overwrite unrelated configuration.
The deprecated `plugin` spelling remains accepted by the current OpenCode schema;
newer configurations can append the same URL to `plugins` instead. Restart
OpenCode and use its native skill tool to load a generated role. Keep the whole
bundle in place: the entry point resolves role paths relative to itself. Do not
copy only `index.js` to a plugin directory.

Without `--out`, the stable destination is
`~/.local/share/pockit/opencode-plugin` or absolute
`$XDG_DATA_HOME/pockit/opencode-plugin`. `--package-name` and `--package-version`
share Pi's validation and defaults. Updates require `--overwrite`; `--check`
includes plugin and metadata bytes. If moving a bundle, remove its old plugin
entry to avoid duplicate registrations. `--install` appends the URL to the user
configuration under `os.UserConfigDir()/opencode/opencode.json` (on Linux,
`$XDG_CONFIG_HOME/opencode/opencode.json` or `~/.config/opencode/opencode.json`).
It preserves unrelated settings and plugin entries, uses an existing `plugins`
key when present (otherwise `plugin`), and skips URLs already registered in either
array. It writes atomically and refuses symlinks and nonregular config files.
Existing `opencode.jsonc` or non-JSON configuration requires manual merging;
PocKit refuses rather than stripping comments or shadowing JSONC settings.

Choose one host output flag per run and use a separate directory for each host.
For automatic user installation, pass `--opencode-plugin --install`, then restart
OpenCode. Instructions appear on stderr and in JSON (`installation.config` and
`installation.note`, with no `installation.command`) only after PocKit has written
all files. Dry runs, checks, and refusals do not report a package as ready to
install. As with Pi, some source failures may still leave a usable package (exit 4).

See upstream [plugins](https://opencode.ai/docs/plugins/),
[skills](https://opencode.ai/docs/skills/), and
[configuration schema](https://opencode.ai/config.json).

## Goose plugin output

`--goose-plugin` creates an Open Plugins bundle with `plugin.json`, `README.md`,
`CHANGELOG.md`, and `skills/ROLE/` directories. Each skill keeps its references,
provenance, scripts, and inert MCP fragments alongside `SKILL.md`. Manifest paths
are relative to the bundle root, under `skills/`. Links within a skill are relative
to that skill's directory. PocKit generates no hooks or executable entry point.

```sh
pockit skills generate --role as-reviewer --goose-plugin --out /private/pockit-goose
# Review the entire bundle before installing.
mkdir -p ~/.agents/plugins/pockit-skills
cp -R /private/pockit-goose/. ~/.agents/plugins/pockit-skills/
```

For project-only discovery, copy to `<project>/.agents/plugins/pockit-skills/`.
Use a Goose version supporting Open Plugins, with its Skills extension enabled.
Start a new session and load `pockit-skills:as-reviewer`. Open Plugins prefixes
skill names with the manifest's plugin name. You need Goose's file tools to read
supporting files, so keep the whole plugin together, including the skill wrappers.

Without `--out`, generation uses `~/.local/share/pockit/goose-plugin` or absolute
`$XDG_DATA_HOME/pockit/goose-plugin`. Package defaults and SemVer validation match
Pi. Goose package names use unscoped lowercase npm-style names, at most 214
characters; `@team/name` is rejected so the name can serve as one plugin directory.
Changing the package name does not change the output path.

After PocKit writes all files, JSON includes `installation.note` and stderr shows
manual copy instructions. There is no command or configuration snippet. Dry runs,
checks, refusals, and interrupted writes do not report the plugin as ready to
install. Some source failures can still leave a usable plugin with exit 4.
`--check` compares manifest, skill, and changelog bytes offline.

`--goose-plugin --install` copies the entire bundle into
`~/.agents/plugins/NAME/` for the current user without editing Goose settings.
To replace an existing installation, pass `--overwrite`. PocKit refuses symlinks,
nonregular files, and overlapping output and installation paths. It copies into
private directories with atomic writes for each file. To update, regenerate with
`--install --overwrite`, then start a new Goose session. Goose's git-backed
`plugin install`, `plugin update`, and auto-update commands do not manage these
local copies. Remove obsolete roles or old plugin copies yourself; generation
and copying leave stale files in place. Without `--install`, follow the manual
copy instructions above.

Upstream sources: [plugins](https://github.com/block/goose/blob/main/documentation/docs/guides/context-engineering/plugins.md)
and [skills](https://github.com/block/goose/blob/main/documentation/docs/guides/context-engineering/using-skills.md).

## Cursor plugin output

`--cursor-plugin` creates `.cursor-plugin/plugin.json`, `README.md`,
`CHANGELOG.md`, and `skills/ROLE/` with skill metadata and all local supporting
files. Cursor discovers skills from this default directory. It has no active
hooks, scripts, or root MCP configuration. Per-role `mcp-servers.json` files stay
inactive, and bundled script snapshots are read-only references.

```sh
pockit skills generate --role as-reviewer --cursor-plugin --out /private/pockit-cursor
# After reviewing the entire bundle:
pockit skills generate --role as-reviewer --cursor-plugin --install --overwrite
```

Without `--out`, output uses `~/.local/share/pockit/cursor-plugin` or absolute
`$XDG_DATA_HOME/pockit/cursor-plugin`. Package defaults are `pockit-skills` and
`1.0.0`. Cursor names must be lowercase kebab-case, at most 64 characters;
scoped names are rejected. Versions use the same SemVer validation as other hosts.

`--install` copies the complete bundle, including the hidden manifest directory,
to `~/.cursor/plugins/local/NAME/`. You do not need a Cursor executable, settings
edits, or marketplace registration for the copy. To replace an existing install,
pass `--overwrite`. PocKit refuses symlinks, nonregular files, and overlapping
output and install paths. Writes are private and atomic per file, but the whole
bundle is not a single transaction. PocKit does not delete obsolete files; handle
updates and cleanup after renaming manually.

Restart Cursor or run **Developer: Reload Window**, confirm skills in
**Customize**, and invoke `/as-reviewer` (or `/ROLE`). Local plugin imports must
be allowed. Enterprise administrators control **Allow Local Plugin Imports**,
which is off by default. A marketplace install with the same name takes
precedence over a local copy. Copying files does not bypass policy or verify
that Cursor loaded them.

Without `--install`, copy the whole output directory to the same local plugin
location yourself. After successful generation, JSON includes `installation.note`
and stderr shows instructions. Dry runs, checks, refused writes, and interrupted
writes do not report the plugin as ready to install. Installation results go to
stderr so stdout remains valid JSON. You cannot combine `--install` with
`--dry-run` or `--check`.

Upstream: [Cursor plugins](https://cursor.com/docs/plugins),
[plugin reference](https://cursor.com/docs/reference/plugins), and
[skills](https://cursor.com/docs/skills).

## Repository documentation bundles

You can bundle upstream documentation for agents with the same fetch, cache,
and generation steps. PocKit preserves source files without rendering Markdown,
summarizing content, or compiling whole repositories. List individual files as
`gh_file_md` or `markdown`. Entries of kind `gh_repository`/`gh_directory` are
pointers; PocKit does not crawl them. MDX stays unchanged, including imports and
components, rather than being rendered as plain Markdown. Each role's bundle
index records the upstream context for relative links. PocKit does not fetch
linked files automatically.

The checked-in `examples/agent-docs/sources.json` and `roles.json` provide separate
`pi-docs`, `opencode-docs`, `goose-docs`, and `cursor-docs` roles covering
extension/package and plugin/skill docs. Cursor uses its published Markdown
pages; the other hosts use repository files on moving branches.
To reproduce docs for a specific release,
replace `main`/`dev` with the relevant commit SHA and add any other pages you need.
The example includes selected pages, not a complete copy of any repository. Keep
licenses and attribution when redistributing upstream documents.

From this checkout:

```sh
pockit sources validate --sources examples/agent-docs/sources.json --roles examples/agent-docs/roles.json
pockit sources plan --sources examples/agent-docs/sources.json --roles examples/agent-docs/roles.json --json
# Review the plan before authorizing remote collection.
pockit sources fetch --sources examples/agent-docs/sources.json --roles examples/agent-docs/roles.json
pockit input --sources examples/agent-docs/sources.json --roles examples/agent-docs/roles.json --opencode-plugin --out /private/agent-docs-opencode
pockit input --sources examples/agent-docs/sources.json --roles examples/agent-docs/roles.json --pi-extension --out /private/agent-docs-pi
pockit input --sources examples/agent-docs/sources.json --roles examples/agent-docs/roles.json --goose-plugin --out /private/agent-docs-goose
pockit input --sources examples/agent-docs/sources.json --roles examples/agent-docs/roles.json --cursor-plugin --out /private/agent-docs-cursor
```

All outputs reuse the same cache and include all four documentation roles; add
`--role pi-docs`, `--role opencode-docs`, `--role goose-docs`, or `--role cursor-docs`
to select just one. Generation never fetches missing pages. To update docs from
moving branches, fetch explicitly with `--refresh`, then regenerate with
`--overwrite` and check for individual source failures. These bundles contain
upstream references, not PocKit's own CLI documentation.

### Nightly GitHub Actions rebuild

[`.github/workflows/nightly-agent-docs.yml`](../.github/workflows/nightly-agent-docs.yml)
runs daily at 03:23 UTC. You can also start it manually with Run workflow.
Scheduled runs use the default branch; the workflow must be present there and
Actions must be enabled. GitHub may delay or disable inactive scheduled workflows.

The job builds PocKit using `go.mod`, validates the example catalog, and refreshes
all eight upstream pages into a fresh private cache in the runner's temporary
directory. It then generates and checks Pi, OpenCode, Goose, and Cursor outputs
from that cache. It does not install agents, execute fetched code, or commit
generated files. If a fetch or generation step fails, the job fails without
publishing a partial bundle. Only Go dependencies are cached across runs;
documentation snapshots are fetched again each time.

Download `docs-bundles-RUN_ID-ATTEMPT` from the run's artifacts and extract its
`agent-docs.tar.gz` into a trusted directory. The archive contains `pi/`,
`opencode/`, `goose/`, `cursor/`, and `recipe/` (catalog, roles, schema, and PocKit commit).
Tar preserves generated permissions; artifacts expire after 14 days.
`docs-reports-RUN_ID-ATTEMPT` contains JSON validation/plan/fetch/generate/check
reports, including diagnostics when a step fails. Report installation paths
refer to the ephemeral runner: install Pi using your extracted `pi/` directory,
and configure OpenCode with the absolute file URL of the extracted
`opencode/index.js`, not the runner URL. Copy the extracted `goose/` bundle into
`~/.agents/plugins/pockit-skills/` (or the project equivalent), then start a new
Goose session. Load `pockit-skills:goose-docs` for Goose's upstream docs.
For Cursor, copy the extracted `cursor/` bundle (including `.cursor-plugin/`)
to `~/.cursor/plugins/local/pockit-skills/`, reload Cursor, and invoke
`/cursor-docs`. Local plugin policy and marketplace precedence still apply.

The read-only GitHub token is available only to the fetch step; no additional
secrets or repository write permissions are required for these public sources.
The workflow uses commit-pinned actions. Edit the example catalog to change which
pages are rebuilt; pin upstream refs to commit SHAs instead of moving branches
when a fixed release snapshot is needed.

## Standalone skills and existing MCP endpoints

### Artifacts and source loading

```text
bundle-state.json               # artifact digests + aggregate fingerprint
ROLE/
├── SKILL.md                    # compact loading guide + labeled local links
├── sources.json                # source contexts + local-bundle plan mappings
├── mcp-servers.json             # inert endpoint fragments
├── bundle-index.json            # URLs, link bases, MIME, content hashes, local_paths
├── references/FULL_SHA256.md    # cached Markdown
├── documents/FULL_SHA256.EXT    # cached documents
└── scripts/
    ├── FULL_SHA256.EXT          # original script snapshots
    └── README.md               # prerequisites, when scripts are present
```

- Each selected role gets its own copies of inherited sources, so you do not
  need a separate base skill. Omit `--role` to generate all configured wrappers.
- `SKILL.md` links use readable catalog labels, falling back to URLs, and are
  sorted by source ID. Read only references relevant to your task. Larger source
  files do not enlarge the wrapper; adding sources adds links.
- Source bodies stay in separate files with their original bytes preserved. They
  are never inlined or summarized.
  `skill_document` goes to `documents/` with MIME-derived `.md`, `.html`, `.json`,
  `.txt`, or unknown `.bin`; no decoding or rewriting.
- PocKit validates source `SKILL.md` name and description metadata without
  repairing it. Original frontmatter stays in the reference and never replaces
  the generated wrapper. Wrappers include valid metadata, discovery information,
  provenance, and warnings about untrusted sources and lack of authority.
- Identical cached inputs yield identical sorted artifact bytes, without generated
  timestamps. Manifests include missing/failed contexts. Unavailable sources and
  pointers have empty `local_paths`; indexes do not advertise `inline_anchor`.
- Source IDs hash URL/identity; `content_sha256` hashes verified cached bytes.

PocKit leaves original relative links unchanged. Resolve them against
`relative_link_base_url` in `bundle-index.json`, not the hashed filename. Strip
fragments for lookup, but keep them for navigation. If no local path is packaged,
the link points upstream only. PocKit does not recursively fetch linked pages,
templates, images, or repository dependencies. Fetching additional content
requires separate authorization.

Bundled scripts are snapshots, so do not assume they can run on their own.
Inspect them, use an authorized source checkout with the original
repository-relative paths and dependencies, and obtain explicit operational
authorization. Changing the working directory will not fix a script that derives
its root from its own location. Generation does not execute scripts, install
dependencies, authenticate, or retrieve secrets.

### Offline update checks

```sh
pockit skills generate --out /private/skills --check --json
# Review changes, then explicitly rebuild.
pockit skills generate --out /private/skills --overwrite --json
```

`bundle-state.json` records SHA-256 digests of expected artifacts and an aggregate
fingerprint, excluding the state file itself. Checks compare the expected bytes
with files on disk, rather than relying on saved state alone.

| Check property | Behavior |
| --- | --- |
| JSON | `fingerprint`, `needs_rebuild`, `changed` paths |
| Rebuild triggers | Missing/modified output, changed cache bytes, provenance, roles/options, or generator output |
| Required options | Explicit `--out`; same roles, host output flag, package name, and version as generation |
| Writes | None; no overwrite permission needed |
| Exit 4 | Stale output or unavailable mappings; distinguish via `needs_rebuild` and `results` |
| Excluded | Unknown/obsolete files, irrelevant raw-input formatting, unused cache entries, unseen upstream edits |

For remote freshness, explicitly run `sources fetch --refresh` first. Checks
never fetch or delete obsolete files.

### MCP fragments

`mcp-servers.json` uses VS Code's fragment shape:

```json
{"servers":{"source-FULL_SHA256":{"type":"http","url":"https://example.com/mcp"}}}
```

These fragments are inactive suggestions. PocKit does not install them, send
endpoint credentials, verify availability, connect, or act as a proxy. Check
client compatibility and authentication before copying them.

## JSON compatibility changes

The former `mcp serve`, stdio protocol, and MCP SDK are removed. Use plans,
generation reports, and bundle files; external endpoint fragments remain inert.

| Previous action/output | Current action/output |
| --- | --- |
| `mcp_document` | `skill_document` |
| `pockit://sources/ID` | `ROLE/sources.json`; find pointer/missing/unsupported entry by ID |
| `pockit://documents/ID` | `ROLE/bundle-index.json`; find ID's `local_paths` (extension depends on cached MIME) |
| Markdown/script paths | Unchanged: `ROLE/references/ID.md`, `ROLE/scripts/ID.EXT` |
| `mcp_connection`, `ROLE/mcp-servers.json` | Unchanged, inert |

Goose plugin mappings add a `skills/` prefix to these paths. Outputs give paths
relative to the bundle root or manifests for lookups; they do not mean uncached
content exists. IDs, provenance, ordering, and report schema version `1` are
unchanged even though action and output meanings have changed. Update consumers
accordingly. `--check` detects changed expected bytes; pass `--overwrite` to
regenerate. PocKit does not edit installed bundles or remove obsolete files.

## Terminal output

| Mode | Output |
| --- | --- |
| Default terminal | Inline ViewKit panels for live progress, result counts, configuration paths, warnings, and host installation instructions; no successful-entry list |
| Fetch / generation | Progress by finished sources / written artifacts (preflight for dry runs), not byte counts |
| 100% | Processing finished; individual sources may have failed, so inspect warnings and the exit code |
| `--json` | Complete plans, outcomes, paths, counts; stdout free of progress and panels; diagnostics remain plain text on stderr |
| `--plain`, `TERM=dumb`, redirection | Plain text without panels or cursor controls; progress appears only as a final bar |
| `NO_COLOR` | Terminal panels without color; live progress still redraws in place |
| Narrow terminals | Panels fit the terminal width, capped at 100 columns; below 12 columns, use text output |
| Terminal safety | Strip source control sequences from warnings; no alternate screen, raw mode, or font probing |

Progress panels redraw only their own lines. They stay in the terminal's normal
scrollback when the command finishes. Progress panels distinguish complete and
incomplete runs; 100% measures processing, not source availability. Help and
`config schema` output remain plain text and raw JSON, respectively.

Run `pockit` for help. Fetch commands authorize collection; generation commands
write offline output. Pass `--overwrite` to allow replacement or `--dry-run` to
preview. Commands do not ask for interactive confirmation. You can cancel an
operation, but completed writes may remain under the partial-write rules.

## Architecture and verification

```text
cli → internal/app session → catalog / inheritance / planning
                          → fetching / cache / skills
ui  → progress rendering (no domain rendering dependency)
```

Each command resolves one plan and passes it directly to fetching or generation.
Tests use temporary directories, fixture catalogs, and local HTTP servers through
a test-only transport. There is no production flag to disable network policy.

```sh
test -z "$(gofmt -l cmd data internal)"
go vet ./...
go test ./...
go test -race ./...
go build ./cmd/pockit
```

For reproducible builds, use `-trimpath -buildvcs=false -ldflags=-buildid=` with
the same Go toolchain, target, CGO settings, and dependencies. The release
procedure compares two builds and validates embedded defaults outside the
checkout. See the [verification notes](verification.md) for safety coverage and
recorded baseline failures. These checks need no live credentials or services.
