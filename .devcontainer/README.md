# Devcontainer

This dev container is generated from
[pi-devcontainer](https://github.com/zorak1103/pi-devcontainer) — a template that pairs a
Go/Node toolchain with the [pi coding agent](https://github.com/earendil-works/pi-coding-agent).
Everything below is set up automatically when a new container is created; for the full
mechanism (configuration layers, mount/volume map, what each script does), see that repo's
docs: [architecture](https://github.com/zorak1103/pi-devcontainer/blob/main/docs/architecture.md),
[extending](https://github.com/zorak1103/pi-devcontainer/blob/main/docs/extending.md),
and [how-to](https://github.com/zorak1103/pi-devcontainer/blob/main/docs/how-to.md).

## What a new container gets

- **Toolchain**: Go (image), Node 22 and mise-managed tools (features), plus `task` and the
  other project tools via `mise install` (`post-create.sh`).
- **pi**: installed at container creation (`install-pi.sh`); global packages, MCP servers,
  models, skills and Claude plugins declared in the personal layer are copied into place and
  fetched eagerly.
- **Persistent caches**: named volumes for the Go module/build caches, mise data, pi's npm
  packages, Claude plugin clones, `~/.config` and shell history — so rebuilds are fast and
  settings survive.
- **Personal layer**: `.devcontainer/.personal/` is regenerated on every container start from
  `~/.pi/devcontainer/` on the host (or `$PI_DC_PERSONAL`, see `sync-personal.js`). It is
  gitignored, never committed: it carries one developer's own pi settings, tools and context
  across every project built from this template, without touching this repo's shared config.
- **API keys**: provider keys (e.g. `ANTHROPIC_API_KEY`) are forwarded from the host
  environment via `devcontainer.json`'s `remoteEnv` — set them on your host machine, not in
  this repo.

## Not part of this repo

The personal layer and all keys are host-local, not committed here. Anyone else opening this
project in the container gets pi's defaults unless they set up their own personal layer
(see pi-devcontainer's [extending](https://github.com/zorak1103/pi-devcontainer/blob/main/docs/extending.md)
and [how-to](https://github.com/zorak1103/pi-devcontainer/blob/main/docs/how-to.md) docs).
