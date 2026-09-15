# Devcontainer

This dev container is generated from
[pi-devcontainer](https://github.com/zorak1103/pi-devcontainer). For the full mechanism
(the three configuration layers, the mount/volume map, what each script does), see that
repo's docs, in particular
[docs/architecture.md](https://github.com/zorak1103/pi-devcontainer/blob/main/docs/architecture.md),
[docs/extending.md](https://github.com/zorak1103/pi-devcontainer/blob/main/docs/extending.md),
and [docs/how-to.md](https://github.com/zorak1103/pi-devcontainer/blob/main/docs/how-to.md).

## Personal layer

`.devcontainer/.personal/` is regenerated on every container start from
`~/.pi/devcontainer/` on the host (or `$PI_DC_PERSONAL`, see `sync-personal.js`). It is
gitignored, never committed: it carries one developer's own pi settings, tools and context
across every project built from this template, without touching this repo's shared config.

## Global pi packages (this machine)

This machine's personal layer (`~/.pi/devcontainer/settings.json`) currently declares:

```json
{
  "packages": [
    "npm:pi-zentui",
    "npm:pi-subagents",
    "npm:@dietrichgebert/ponytail",
    "npm:pi-mcp-adapter",
    "npm:pi-claude-marketplace"
  ]
}
```

`post-create.sh` fetches all of them eagerly during container creation (`pi update
--extensions`, right after the personal `settings.json` is copied into place).

### pi-zentui

If `~/.pi/devcontainer/zentui.json` exists, it is copied to `~/.pi/agent/zentui.json` so a
`/zentui` configuration survives rebuilds.

### pi-mcp-adapter: Context7

`~/.pi/devcontainer/mcp.json` declares an MCP server, copied to `~/.pi/agent/mcp.json`:

```json
{
  "mcpServers": {
    "context7": {
      "url": "https://mcp.context7.com/mcp",
      "headers": { "Authorization": "Bearer ${CONTEXT7_API_KEY}" }
    }
  }
}
```

`CONTEXT7_API_KEY` reaches the container through `devcontainer.json`'s `remoteEnv`, set once
on the host with `setx CONTEXT7_API_KEY <key>` (see
[setup-windows.md](https://github.com/zorak1103/pi-devcontainer/blob/main/docs/setup-windows.md#the-api-key)
for the same pattern applied to the provider key).

### pi-claude-marketplace: claude-plugins-official

`~/.pi/devcontainer/claude-plugins.json` declares the marketplace and one plugin, copied to
`~/.pi/agent/claude-plugins.json` and reconciled automatically at pi's own session start (no
`/claude:plugin` command needed):

```json
{
  "schemaVersion": 1,
  "marketplaces": {
    "claude-plugins-official": { "source": "anthropics/claude-plugins-official", "autoupdate": true }
  },
  "plugins": { "commit-commands@claude-plugins-official": {} }
}
```

`superpowers` is deliberately **not** in that declarative list: it needs `--partial`
(unsupported hooks), which the declarative config cannot express (see
[findings.md#f17](https://github.com/zorak1103/pi-devcontainer/blob/main/docs/findings.md#f17--claude-pluginsjson-cannot-declare-a-partially-installable-plugin)).
To get it in this container, run once, interactively:

```text
/claude:plugin install --partial superpowers@claude-plugins-official
```

The resulting record persists in the `pi-dc-${localWorkspaceFolderBasename}-claudeplugins`
volume until that volume is removed.

## Not part of this repo

Everything above is host-local, not committed here. Anyone else opening this project in the
container gets pi's defaults unless they set up the same personal layer on their own machine.
