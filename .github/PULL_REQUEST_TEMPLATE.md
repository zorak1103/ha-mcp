## Related issue

<!-- Every PR must reference an existing issue. Example: Fixes #123 -->
Fixes #

## What changed

<!-- Short description of the change -->

## Testing

<!-- How did you verify this? Unit mocks alone have missed real bugs in this project before
     (e.g. script `sequence` attribute, zone/person WS command prefixes) — call out if you
     ran the integration suite against a real Home Assistant instance. -->

- [ ] `task test` passes
- [ ] `task test:integration` run against a real Home Assistant instance (or N/A — explain why)

## Checklist

- [ ] I discussed the approach on the linked issue before starting (or this is a trivial fix: typo, docs, obvious one-liner)
- [ ] This PR is focused on a single change
- [ ] Tests were written before the implementation (TDD) and cover the new behavior
- [ ] `task fmt:fix` and `task lint` pass
- [ ] If this adds/changes a tool: integration test added/updated in `internal/handlers/integration/`
- [ ] If this adds a new tool: docs updated per the Documentation Update Checklist in `CONTRIBUTING.md` (README, `docs/tools.md`, `docs/architecture.md`, `docs/feature-comparison.md`, `CLAUDE.md`, `docs/integration-tests.md`, relevant `.claude/skills/ha-mcp/*/SKILL.md`)
