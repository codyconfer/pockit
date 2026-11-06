# Real-host package integration

Run `make test-integration` from the repository root. You need Docker with
Linux/amd64 support, using emulation on other hosts. The tests do not use agents
installed on your machine, API keys, or mounted home directories. The build
downloads public tools; the tests run as a non-root user with `--network none`,
all capabilities dropped, and `no-new-privileges`. They do not publish or upload
anything to an agent service.

The build also runs formatting checks, `go vet ./...`, `go test ./...`, and
`go test -race ./...` without skips. It compiles the tagged Go integration test
into the image. You can run ordinary `go test ./...` without installing agents.

## Coverage

Each host gets an isolated HOME/XDG configuration, an explicit two-role catalog
with inheritance, and a verified local cache fixture. Tests run the built
`pockit` binary with `--install`, then upgrade with `--overwrite` and version 1.1.0.
They check manifests and frontmatter, original reference, document and script
bytes, mixed-kind mappings, inherited resources, MCP pointers, and complete
installed copies, including hidden files. They also check deterministic
`--check` results and refusal to replace files without `--overwrite`. The tests
never execute script snapshots or fetch live sources. The Go suite covers HTTP
validation and fetching separately.

| Host | Real-host check |
| --- | --- |
| Pi | `pi list`, installed-package SDK loader, extension lifecycle `resources_discover`, and both discovered skill paths |
| OpenCode | `opencode debug skill` discovers both skills at their installed locations |
| Goose | `goose skills list` discovers both skills at the installed plugin paths |
| Cursor | Real `agent --version`/`--help`, installed hidden manifest, all skill resources, and upgrade/copy contract |

Cursor desktop handles plugin activation, and the headless CLI has no documented
unauthenticated command for discovering plugins. This suite does not test desktop
activation or model invocation for any host.

## Versions and CI

`integration/Dockerfile` pins the versions used for compatibility tests: Pi 1.5.0,
OpenCode 1.19.5, Goose 1.57.0, and Cursor CLI 2026.11.05-86047df. The build verifies
Goose and Cursor archives with SHA-256 instead of running remote installer
scripts. Use Docker build args to change versions; when changing an archive,
update its hash too. The listed versions are the ones used in these tests, not
the minimum supported versions.

`.github/workflows/package-integration.yml` runs on pull requests, pushes to main,
and manual dispatch. It keeps build and test logs for seven days, including logs
from failed runs. The workflow has read-only repository permissions and passes
no credentials into the build or container. After building, you can debug one
host at a time:

```sh
docker run --rm --network none --cap-drop ALL \
  --security-opt no-new-privileges pockit-integration \
  -test.v -test.run '^TestInstalledPackages/pi$' -test.timeout=3m
```
