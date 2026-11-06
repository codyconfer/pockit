# Compact skill output verification

`SKILL.md` has a short loading guide with links to local source files. Catalog
context labels help you choose references without first opening the larger
provenance manifests. The source files keep their complete original bytes.

## Measured change

These measurements were taken after the compact-entrypoint change, before
resource serving was removed. The generator has since changed local planning
mappings and shortened the loading guide. The totals below have not been
remeasured since those changes. See
[JSON compatibility changes](reference.md#json-compatibility-changes).

Both generator versions used the same `data/sources.json`, `data/roles.json`, and
local cache snapshot, with no changes to the cache or source fetches. The numbers
measure the initial entrypoint load, excluding references read afterward.

| Role | Before bytes | After bytes | Before estimated tokens | After estimated tokens | Reduction |
| --- | ---: | ---: | ---: | ---: | ---: |
| as-grafana-engineer | 11,161 | 1,275 | 2,791 | 319 | 88.6% |
| as-reviewer | 39,856 | 1,781 | 9,964 | 446 | 95.5% |
| Total | 51,017 | 3,056 | 12,755 | 764 | 94.0% |

Token estimates use `ceil(UTF-8 bytes / 4)`, rounded independently for each row.
This is a rough estimate rather than a model tokenizer's count. Actual savings
vary with the model, source content, cache availability, and selected roles.

The nine packaged source files, including inherited copies, were byte-identical
between versions. `sources.json` and `mcp-servers.json` were unchanged too. Every
generated entrypoint link resolved locally. The index still includes content
hashes and local paths, but no longer lists the removed inline anchors.

## Reproduction

Keep a binary for each generator version and use one unchanged cache snapshot.
Run both binaries with the same explicit inputs, writing to separate output
directories:

```sh
pockit-before skills generate --sources data/sources.json --roles data/roles.json \
  --cache /private/cache-snapshot --out /private/before --json
pockit-after skills generate --sources data/sources.json --roles data/roles.json \
  --cache /private/cache-snapshot --out /private/after --json
wc -c /private/before/*/SKILL.md /private/after/*/SKILL.md
pockit-after skills generate --sources data/sources.json --roles data/roles.json \
  --cache /private/cache-snapshot --out /private/after --check --json
```

Repeat with `--pi-extension` to measure Pi packages. When the new generator checks
old output, it must report `needs_rebuild: true`. Pass `--overwrite` to regenerate
in the new format. Neither generation nor checks make network requests.

## Regression evidence

- `TestCompactEntrypoints`: ordinary and Pi bundles, readable labels, local links,
  exact source bytes, inherited copies, safety directives, empty roles, compact
  metadata, deterministic artifacts and fingerprints.
- `TestCompactEntrypointIndependentOfSourceSize`: changing a source from one byte
  to 100,000 bytes does not change `SKILL.md`.
- `TestSourceLabels`: deterministic, escaped, bounded Unicode labels and fallbacks.
- `TestLinkedMarkdownAndOfflineUpdates`: original Markdown (including CRLF,
  frontmatter and empty files), cache updates, clean/stale checks, output edits,
  overwrite rebuilds and symlink refusal.

All skill tests pass under the race detector except the known
`TestSevenSelfContainedDeterministicSkills` fixture mismatch. CLI, app, and config
tests pass, as does `go vet
./...`. The full suite still reports the four existing fixture-count failures
in catalog, inheritance, planning, and skills. No new failures were observed.
