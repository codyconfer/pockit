# Verification and safety evidence

All tests use fixtures. These checks did not fetch production sources, execute
downloaded code, install agents automatically, or connect to external MCP servers.

## Covered contracts

| Package | Tested contracts |
| --- | --- |
| `internal/catalog` | Bundled schema/defaults; Draft 2020-12 URI/date assertions; duplicate nonempty URLs; empty/unknown kinds; exact optional metadata; external-schema refusal; credential URL redaction before diagnostics |
| `internal/inheritance`, `internal/planning` | Role selection; unknown roles/parents and cycles; deterministic identities/resolution; conflicting direct/inherited metadata; every context/disposition and local-bundle mapping |
| `internal/fetching` | Test-only local transport; GitHub raw mapping/auth headers; no credentials to unrelated hosts; once-per-URL attempts, including failures; cache reuse; login/auth/HTML, binary/size/timeout/redirect failures; bounded concurrency/cancellation; production denial of local/private/reserved addresses |
| `internal/cache` | Private permissions; shared-root refusal without chmod; content identity/corruption checks; no false-success corrupt refresh; atomic no-clobber writes; traversal/symlink refusal |
| `internal/skills` | Independent/inherited references; exact source bytes; deterministic artifacts; source SKILL metadata validation/preservation; no-write previews; overwrite/path guards; complete inert mappings; Pi selected-role discovery, manifests, and install commands; OpenCode deterministic ESM, URL escaping, config merging/idempotency, and install-readiness guards; Goose manifest, nested skill layout, namespaced installation notes, and safe package naming |
| `internal/cli` | Progress/JSON/plain output; diagnostics/exits; input overrides; policy refusal; offline preview/write/overwrite; Pi/OpenCode/Goose instructions and flag validation; offline three-host documentation recipe; removed-command rejection; built subprocess offline plan outside checkout; one resolved plan reused for fetch/generation |
| `internal/ui` | Progress write failures; terminal-escape sanitization; styled/plain output without full-screen controls |

Reports leave out URLs containing secrets, even when duplicate URLs cause a
failure. Diagnostics do not store raw remote error bodies or authorization headers.

## Release checks

Run from the checkout:

```sh
test -z "$(gofmt -l cmd data internal)"
go vet ./...
go test ./...
go test -race ./...
go build ./cmd/pockit
```

Local release checks also compare the imported catalog and schema byte for byte
with the supplied Desktop originals. CI builds twice with the same toolchain and
reproducibility flags, compares the binaries, then runs validation from another
working directory. Tests create private synthetic Markdown only in temporary
directories. The project contains no private remote content.

## Simplification verification and baseline

Before simplification removed the serving interface, formatting and
`go vet ./...` passed. Both `go test ./...` and `go test -race ./...` reported the
same four existing failures:

| Package / test | Baseline diagnostic |
| --- | --- |
| `internal/catalog` / `TestDefaults` | `roles=2` |
| `internal/inheritance` / `TestDefaults` | `as-reviewer inherited=7` |
| `internal/planning` / `TestEveryContextAccountedFor` | `contexts=20 want=13` |
| `internal/skills` / `TestSevenSelfContainedDeterministicSkills` | `wrappers=2` |

The catalog and fixture expectations do not match in these tests. Expectations
were not loosened to make them pass. Fixture-based tests for compact output,
document and script bundles, updates and checks, Pi packaging, configuration,
installation, and the CLI help separate regressions from these baseline failures.
The added `TestLocalBundleMappings` checks local plan outputs, cache-state lookup,
inherited provenance, actions, and summary counts. See
[JSON compatibility changes](reference.md#json-compatibility-changes) for the
intentional changes to actions and outputs and the removed serving interface.

After simplification, formatting, vet, and a temporary binary build with
reproducibility flags pass. Uncached full tests (`go test -count=1 ./...`) and
race tests (`go test -race -count=1 ./...`) report exactly the four baseline
failures above, with no new failures or race reports.
`TestResolvedLocalPlanOfflineBundle` uses one resolved selected-role plan for
ordinary output, Pi output, and clean checks. It verifies inherited provenance,
local mapping targets, unchanged plan bytes, and byte-exact Markdown, HTML, text,
JSON, and script sources. Tests also pass for compact wrappers and links,
configuration, custom package names, versions and changelogs, mocked installation,
input isolation, staleness, overwrite, and filesystem and network safety.

The reference audit finds former serving identifiers only in the JSON migration
notes. No code or public documentation still refers to the removed font settings
or font probing. Endpoint kinds and inert `mcp-servers.json` fragments are still
supported. These checks did not modify or delete installed bundles or existing
output directories, or fetch production sources.

## OpenCode output verification

`go test ./internal/cli ./internal/skills -run 'Test(OpenCode|Pi|DefaultHasNoPi)' -count=1`
passes. OpenCode tests check deterministic package bytes, identity and version
validation, installation readiness after dry runs, checks and refusals, overwrite
and symlink guards, default and explicit destinations, JSON and plain diagnostics,
and manual-install defaults. When Node is available, the runtime test imports the
generated ESM and checks its only hook (`config`), both skill-config forms,
idempotency, and preservation of unrelated permissions, MCP settings, and URLs.
Without Node, the test reports a skip. No OpenCode binary was available, so these
checks did not include an end-to-end test in the OpenCode host.

Validation and planning succeed for the example documentation catalog's four
fetchable references across two roles without fetching them. Its upstream
Markdown and MDX URLs were checked separately; their content was not installed or
bundled into this checkout. The full suite still reports exactly the four
baseline fixture failures above, with no new failures.

## Goose output and automated documentation verification

```sh
go test ./internal/cli ./internal/skills -run 'Test(AgentDocs|Goose|OpenCode|Pi|Agent)' -count=1
go test -race ./internal/cli ./internal/skills -run 'Test(AgentDocs|Goose|OpenCode|Pi|Agent)' -count=1
```

Both commands pass, as do formatting and `go vet ./...`. Goose tests cover the
Open Plugins manifest and `skills/ROLE/` layout, valid skill metadata, deterministic
bytes, and version-change checks. They also check installation readiness after
dry runs, checks and refusals, overwrite and symlink refusal, package-name
validation, default and explicit output paths, JSON and plain instructions, and
`input` support. PocKit rejects conflicting output flags and combinations of
`--install` with `--dry-run` or `--check`.

`TestAgentDocsBundles` validates the example catalog and fills the cache with
fixture bytes for its six files across three roles. It generates and checks Pi,
OpenCode, and Goose bundles offline, then verifies that each role's references
keep the fixture bytes. The nightly workflow refreshes those six upstream files
and generates and checks all three host formats before archiving them. These
local checks did not run GitHub Actions or install Goose. Compatibility is based
on the upstream plugin and skill specifications.

The full `go test ./...` run still fails in the same four baseline tests. With the
current embedded catalog, their diagnostics are `roles=3`,
`as-reviewer inherited=6`, `contexts=19 want=13`, and `wrappers=3`, respectively.
The baseline values recorded earlier reflect the catalog at that time. Goose
support did not change fixture expectations or embedded inputs.

## Automatic host installation

`--install` supports OpenCode and Goose as well as Pi. Fixture tests cover
user-config creation, preservation of unrelated settings and plugin entries,
both OpenCode plugin-key spellings, and idempotent registration. They also test
refusal of malformed JSON and JSONC, whole-bundle Goose copies, named installs,
updates, and refusal of symlinks, nonregular files, and overlapping paths. These
tests do not invoke OpenCode or Goose; they install only into temporary user
directories. JSON stdout contains the generation report. Installation results
and manual fallback instructions go to stderr, and installation errors leave
the generated output in place.

`go test ./internal/cli` and `go vet ./...` pass. Full tests retain the same four
catalog/fixture failures with the current diagnostics listed above.

## Cursor output, local installation, and documentation automation

```sh
go test ./internal/cli ./internal/skills -run 'Test(Cursor|InstallHost|InstallCursor|AgentDocs)' -count=1
go test -race ./internal/cli ./internal/skills -run 'Test(Cursor|AgentDocs|Goose|OpenCode|Pi|Agent|Install)' -count=1
```

Both commands pass, as do formatting and `go vet ./...`. Tests cover the hidden
`.cursor-plugin/plugin.json` manifest, deterministic nested skill artifacts,
unchanged mapping paths, version-change checks, and installation readiness after
dry runs, checks and refusals. They also check metadata and package validation,
default and explicit output paths, JSON and plain output, `input` behavior,
named local installs and updates, byte-exact copies including hidden files, and
symlink, overlap, and overwrite refusal. Installation tests use temporary home
directories. No real Cursor host was installed or launched.

The agent-docs fixture recipe covers eight pages across four documentation roles
and generates and checks all four host formats. Cursor's two sources are its
published `plugins.md` and `skills.md` pages. The workflow's shell syntax is
checked offline, but GitHub Actions itself was not run. Cursor discovery and
policy instructions come from the upstream plugin documentation; they were not
verified in a live host.

`go test -count=1 ./...` passes all CLI tests and retains exactly the four
pre-existing catalog/fixture failures: `roles=3`, `as-reviewer inherited=6`,
`contexts=19 want=13`, and `wrappers=3`. No embedded catalog or baseline test
expectations were changed for Cursor support.

## Package output adapters

`internal/skills/package.go` defines `PackageAdapter`, which handles host identity,
skill layout, name validation, package artifacts, and installation descriptions.
`PackageAdapterFor` resolves the existing flags at the boundary. Plain skills and
AGENT.md have no package adapter. The builder uses the same adapter for mapping
and artifact paths. Each host's renderer stays in its own file; shared bundle
fingerprints and filesystem safety are unchanged.

`internal/cli/package.go` handles host installation and diagnostics: running the
Pi subprocess, merging OpenCode config, and copying local Goose and Cursor
bundles. To add a host, register a skills adapter, add a CLI installer entry, and
add its boundary flag. The generation and installation lifecycles need no
host-specific branches. Existing flags, package defaults, output paths, and JSON
shapes are unchanged.

Adapter tests cover all 32 output-flag combinations and host-specific name rules.
They compare renderers and installation descriptions, check deterministic builds,
unchanged input mappings and matching layout prefixes, and verify manual and
failure instructions. `go test ./internal/cli` and focused adapter and host tests
pass. The race tests all pass when the four documented baseline tests are
explicitly skipped:

```sh
go test -race -count=1 ./... -skip '^(TestDefaults|TestEveryContextAccountedFor|TestSevenSelfContainedDeterministicSkills)$'
```

Without filtering, the suite reports exactly those four fixture failures with
the diagnostics listed above. Formatting, vet, a temporary binary build, and
active LSP checks of all six changed Go files pass. Fixture expectations were
not changed.

## Limits deliberately retained

- Login-page detection uses heuristics and cannot prove authenticated access.
- PocKit does not crawl recursively, clone repositories, execute scripts, or
  retrieve live pointer data. Unsupported sources are reported as failures.
- Same-origin-only redirects may reject otherwise useful documentation redirects;
  provide the intended public HTTPS origin directly instead of relaxing policy.
- Progress counts completed outcomes rather than bytes. You can cancel operations,
  but there is no separate UI command, menu, or full-screen navigation. For
  automation, request `--json` explicitly.
- Cache and generated artifacts are unencrypted. Use a trusted private filesystem.
  An existing cache root must already be private; PocKit does not silently change
  its permissions. Digest checks detect corruption, but cannot detect malicious
  replacement by the same OS user.
- Each artifact write is atomic, but the whole tree is not updated in one
  transaction. Cancelling a job may leave earlier completed files. Replacing
  them still requires explicit overwrite.
- These tests do not verify whether cataloged production services, endpoints, or
  credentials work today. Inheritance does not grant authority to use them.
