# CLAUDE.md

Project-level guidance for Claude Code (and other AI coding assistants) working in this repository.

## This repo is a fork

[Z-M-Huang/CLIProxyAPI](https://github.com/Z-M-Huang/CLIProxyAPI) is a soft fork of [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI). **Read [`CONTRIBUTING.md`](./CONTRIBUTING.md) before making changes** — it covers branch model, upstream-sync workflow, customization surface, and release process.

## Hot rules (don't get these wrong)

1. **Branches**: cut from `dev`. Open PRs against `dev`, not `main`. `main` is fast-forwarded from `dev` at release time.
2. **Tags**: `zmh-vX.Y.Z`. Never push a bare `vX.Y.Z` tag — that namespace belongs to upstream.
3. **Upstream**: `router-for-me/CLIProxyAPI` is the `upstream` remote. We selectively merge / cherry-pick from `upstream/dev`. We do **not** open PRs against `router-for-me/*`.
4. **Workflows**: this fork carries **no** GHA workflows. If a sync from upstream brings any back, delete them in the same merge.
5. **Customization conflicts**: when merging from `upstream/dev`, conflicts in the files listed in `CONTRIBUTING.md` are expected — keep our version. Conflicts elsewhere are a refactor signal — investigate, don't paper over.
6. **Docker**: image is `zhironghuang/cli-proxy-api` on Docker Hub, pushed manually from the local machine via `docker buildx build --push`. No `DOCKERHUB_*` GitHub secrets exist on the fork.
7. **Frontend bundle**: served from a separate fork release at `Z-M-Huang/Cli-Proxy-API-Management-Center`. The backend's auto-updater fetches it from there on startup; the Dockerfile does **not** bake `static/management.html`.

## Build / test commands

See `AGENTS.md` for the canonical command list (`gofmt`, `go build`, `go test`, etc.) and architectural notes that apply equally to all assistants.

## Pointers

- `CONTRIBUTING.md` — fork workflow (the source of truth for the rules above).
- `AGENTS.md` — architecture, code conventions, build commands.
