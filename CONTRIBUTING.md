# Contributing to ha-mcp

Thanks for considering a contribution. This project is a Go MCP server for Home
Assistant with fairly strict testing and linting expectations — this doc covers
what CI and the maintainer will actually check.

## Prerequisites

- Go 1.26+
- [`task`](https://taskfile.dev/#/installation) — this project has **no Makefile**, `task` is the only supported entry point
- golangci-lint v2
- Docker (optional, for building/testing the container image)

```bash
git clone https://github.com/zorak1103/ha-mcp.git
cd ha-mcp
task install-hooks   # installs the pre-commit hook (auto-fixes gofmt on commit)
task --list          # see all available tasks
```

## Workflow

1. Open or find an issue describing the change, and get alignment on the
   approach before starting — unless it's a trivial fix (typo, docs, an obvious
   one-liner). If the scope or approach isn't settled yet, start a
   [Discussion](https://github.com/zorak1103/ha-mcp/discussions/categories/ideas)
   instead of an issue.
2. Create a feature branch off `main`.
3. **Write tests first (TDD is required for this project)**: red → green →
   refactor. See `CLAUDE.md`'s Testing Rules for the coverage bar (80% per file)
   and what's exempt.
4. Implement the change.
5. Run the checks below locally.
6. Open a PR using the provided template, linking the issue.

## Running checks

```bash
task build              # go build -o ha-mcp[.exe] ./cmd/ha-mcp
task test                 # go test ./... (Windows-safe, no race detector)
task test:race          # go test -race ./... (Linux/CI)
task test:coverage      # race + coverprofile + per-file 80% enforcement
task lint                # golangci-lint run --timeout=5m ./...
task fmt:fix             # gofmt -w .
task vulncheck           # govulncheck ./...
```

All of the above run in CI; a PR won't be merged if any fail.

### Integration tests

Tool handlers are also covered by integration tests against a real Home
Assistant instance (`internal/handlers/integration/`). If you're changing a
tool's behavior (not just internal refactoring), add or update the
corresponding integration test.

```bash
export HA_INTEGRATION_TEST_URL=http://homeassistant.local:8123
export HA_INTEGRATION_TEST_TOKEN=<your-token>
task test:integration

# or, using an env file:
set -a && source .env.integration && set +a && task test:integration
```

**Safety:** every test entity is created with a `mcptest_<uuid>_<name>` prefix
and cleaned up afterward — this is what makes it safe to point the suite at a
real (ideally non-production) Home Assistant instance. Don't remove or bypass
the prefix helpers (`GenerateTestID`, `BuildEntityID`) when writing new tests.

Some tools genuinely can't be integration-tested (e.g. blueprint `import` needs
a reachable public URL, or operations that would irreversibly affect a
production instance) — see CLAUDE.md's Testing Rules for the accepted
exceptions.

## Code style / linter rules

`golangci-lint` enforces several rules that are easy to trip on the first PR —
`funlen` (60-line function limit), `gocognit` (cognitive complexity), `goconst`
(extract strings repeated 3+ times to constants), lowercase error messages, and
a few Go-idiom rules (no shadowing built-ins like `min`/`max`, no shadowing
imported package names). The full, current list with examples lives in
`CLAUDE.md`'s **Coding Rules** section — read it before your first PR if you're
touching handler code; it will save review round-trips.

## Adding a new tool

New tools have a documentation checklist beyond just code + tests — update all
of the following if applicable (see `CLAUDE.md`'s Workflow Preferences for the
full list): `README.md` (tool count and summary table), `docs/tools.md`,
`docs/architecture.md` (if adding a handler file), `docs/feature-comparison.md`,
`CLAUDE.md` (Key Files / Consolidated Tools sections),
`docs/integration-tests.md`, and the relevant
`.claude/skills/ha-mcp/*/SKILL.md` files.

## Commit messages

Conventional commits: `<type>: <description>`, where type is one of `feat`,
`fix`, `refactor`, `docs`, `test`, `chore`, `perf`, `ci`.

## Pull requests

- Use the PR template — it links the checks above.
- Keep PRs focused on a single change.
- Every PR must reference an issue (`Fixes #123`), except trivial fixes.

## Reporting a security vulnerability

Please don't open a public issue for a security vulnerability — see
[SECURITY.md](SECURITY.md).
