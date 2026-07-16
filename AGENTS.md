# AGENTS.md

## What this is

A CoreDNS external plugin (`discovery`) providing DNS-based service discovery with pluggable sources. Single Go package, flat structure — all source files live in the root `package discovery`.

## Commands

```bash
task test          # go test -race with coverage
task lint          # golangci-lint
task lint-files    # shellcheck, yamllint, jq, markdownlint
task build         # go build ./...
task vet           # go vet
task coverage      # tests + HTML coverage report
task setup-hooks   # configure git pre-push hook (.githooks)
task security      # gosec + govulncheck
task secrets       # gitleaks scan
```

Run a single test: `go test -race -run TestStore_Register ./...`

`task zen` launches the QEMU VM and opens Zed remotely. `task opencode` launches opencode TUI inside the VM. `task git-sub-module` bumps the `.devenv` submodule.

## Code style

`goimports` local-prefixes is `github.com/tdesaules/coredns-service-discovery` (`.golangci.yml`). Group local imports separately from third-party.

## Conventional commits

Commits must follow [conventional commits](https://www.conventionalcommits.org/) — semantic-release uses them to auto-determine version bumps.

- `feat:` → minor (v1.1.0)
- `fix:` → patch (v1.0.1)
- `feat!:` or `BREAKING CHANGE:` → major (v2.0.0)
- `chore:`, `docs:`, `refactor:` → no release

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`.

A pre-push hook (`.githooks/pre-push`) validates commit format and runs gitleaks. Install with `task setup-hooks` after cloning.

## Architecture

- `store.go` — thread-safe in-memory store (`sync.RWMutex`). Keyed by `"<service>.<namespace>"`.
- `source.go` — `Source` interface + global registry. Sources self-register via `init()` → `RegisterSource()`.
- `handler.go` — `ServeDNS`: parses query labels, serves A/SRV from store, falls through to Next on no match.
- `setup.go` — `plugin.Register("discovery", setup)`, Corefile parsing, source lifecycle (OnStartup/OnShutdown).
- `fallthrough` follows CoreDNS semantics via `fall.F`: when configured without zones it forwards all no-match queries to `Next`; with explicit zones only those zones fall through. Without `fallthrough`, no-match queries return NXDOMAIN. SRV queries are filtered by the `_tcp`/`_udp` protocol label.

## Adding a source

1. Create `source_<name>.go` in package `discovery`.
2. Add `func init() { RegisterSource("<name>", func() Source { return &MySource{} }) }`.
3. Implement `ParseConfig(c *caddy.Controller)` — cursor is after `{`; consume tokens with `c.Next()` until `}`.
4. Implement `Run(ctx, store)` — populate store via `store.Register`/`store.Deregister`, block until `ctx.Done()`.

No core files change when adding a source.

## Source sub-block parsing quirk

Caddy v1's `NextBlock()` doesn't handle nested sub-blocks correctly (nesting counter breaks). The setup code works around this: the parent consumes `{` via `NextArg()`, then the source's `ParseConfig` iterates with `c.Next()` (not `NextBlock()`) and breaks on `}`. Follow this pattern — do not use `NextBlock()` inside `ParseConfig`.

Reference implementation from `setup_test.go`:

```go
func (m *mockSource) ParseConfig(c *caddy.Controller) error {
    for c.Next() {
        if c.Val() == "}" {
            break
        }
    }
    return nil
}
```

## DNS convention

Zone is normalized with trailing dot via `plugin.Name(zone).Normalize()`. Query matching is case-insensitive.

- A service:   `<service>.<namespace>.<zone>`
- A instance:  `<instance>.<service>.<namespace>.<zone>`
- SRV:         `_<service>._<proto>.<namespace>.<zone>`

Namespace defaults to `default`. Only A and SRV are handled; all other types fall through.

## Integration repo

This plugin is compiled into CoreDNS via a separate repo `github.com/tdesaules/coredns` (Method 2: external Go program that imports CoreDNS as a library). This repo is pure plugin code — no Dockerfile, no CoreDNS binary.

## CI/Release

A single GitHub Actions workflow (`.github/workflows/release.yml`) runs on push to `main` (code files only — docs, `.devenv/`, `example/` are excluded via `paths-ignore`).

Quality gates (parallel): commitlint, golangci-lint, go test -race + coverage (Codecov + artifact), go build + vet, gosec + govulncheck, gitleaks, file linting (shellcheck, yamllint, jq, markdownlint).

Release job (after all gates pass): semantic-release analyzes conventional commits, bumps semver version, creates git tag `vX.Y.Z`, updates `CHANGELOG.md`, creates GitHub Release. No-op if no `feat:`/`fix:` commits since last release.

Go version is read from `go.mod` via `go-version-file`. Dependabot tracks the `.devenv` gitsubmodule (daily) and GitHub Actions versions (weekly), both auto-merge. Go module bumps are not automated.

GitHub Push Protection should be enabled manually: Settings → Code security → Push protection.

## Dev environment (QEMU VM)

A disposable Fedora CoreOS VM is available via git submodule at `.devenv/` (repo: [qemu-dev-env](https://github.com/tdesaules/qemu-dev-env)).

```bash
# From the repo root (important: PWD determines the 9p share path)
task -t .devenv/taskfile.qemu.yaml check   # verify host deps
task -t .devenv/taskfile.qemu.yaml up       # build + launch VM
task -t .devenv/taskfile.qemu.yaml ssh      # SSH into VM
task -t .devenv/taskfile.qemu.yaml down     # destroy VM
```

The repo root is mounted inside the VM at `~/workspace` via 9p. The VM has Go, task, golangci-lint, gitleaks, and opencode pre-installed via mise (configured in `.devenv/config.k`, gitignored). opencode config (`opencode.json`, `tui.json`, `auth.json`) is read from the host at build time and baked into the ignition config as inline files — the VM gets its own writable copies on the local filesystem.

`task zen` launches the QEMU VM and opens Zed remotely. `task opencode` launches opencode TUI inside the VM.
