# Contributing to Z-M-Huang/CLIProxyAPI

This is a soft fork of [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI). The fork carries fork-specific features on top of upstream while continuing to absorb upstream improvements over time.

If you're a human collaborator or an AI coding assistant, **read this before opening a branch or PR**. Shared assistant guidance lives in [`docs/ai-assistant-guidance.md`](./docs/ai-assistant-guidance.md), which is imported by [`AGENTS.md`](./AGENTS.md) and [`CLAUDE.md`](./CLAUDE.md).

## Goals

1. **Ship our own features** under the `zhironghuang/cli-proxy-api` docker image and `zmh-vX.Y.Z` git tags. This is the primary work.
2. **Track upstream's core improvements** so we benefit from their maintenance without duplicating it. We do this by selectively merging or cherry-picking from `upstream/dev` into our `dev`.
3. **Stay non-disruptive to upstream.** We don't open PRs against `router-for-me/*` — upstream merge is no longer the goal. (See plan history for context.)

## Branch model

```
main  ←─[fast-forward]──  dev  ←─[merge]──  feat/<your-feature>
                          ▲
                          └─ periodic merge / cherry-pick from upstream/dev
```

- **`main`** is the default browse-branch. Always equals (or trails by a tag) `dev`. Don't commit to `main` directly; fast-forward from `dev` when releasing.
- **`dev`** is the integration branch. All feature work and upstream sync land here.
- **`feat/<short-name>`** is where you do your work. Cut from `dev`, PR back into `dev`.

## Workflow: starting a new feature

```bash
git fetch origin
git checkout -b feat/your-thing origin/dev
# ... commit ...
git push -u origin feat/your-thing
gh pr create --repo Z-M-Huang/CLIProxyAPI --base dev --head feat/your-thing
```

Open the PR against **`dev`**, never `main`. After merge, the feature branch can be deleted.

## Workflow: absorbing upstream changes

We track `router-for-me/CLIProxyAPI` as the `upstream` remote. We do **not** blindly merge `upstream/dev` — we look at what's new and pick what fits.

```bash
git fetch upstream
# What's new since the last sync?
git log --oneline origin/dev..upstream/dev

# Either: cherry-pick selected commits onto a sync branch
git checkout -b sync/upstream-YYYY-MM-DD origin/dev
git cherry-pick <sha1> <sha2> <sha3>

# Or: full merge if it's mostly low-conflict
git checkout -b sync/upstream-YYYY-MM-DD origin/dev
git merge --no-ff upstream/dev
# Resolve conflicts ONLY in the customization surface below.

git push -u origin sync/upstream-YYYY-MM-DD
gh pr create --repo Z-M-Huang/CLIProxyAPI --base dev --head sync/upstream-YYYY-MM-DD \
  --title "sync: pull upstream/dev YYYY-MM-DD"
```

Conflicts during a sync are expected only in the customization surface below. **If you hit a conflict outside that list, treat it as a refactor signal — investigate before resolving.**

## Customization surface

These are the files where the fork diverges from upstream. When syncing upstream, conflicts here are normal — keep our version. Everywhere else, take upstream's.

- `Dockerfile` — fork image build metadata plus optional `MANAGEMENT_PANEL_RELEASE_URL` pinning for prerelease UI test images.
- `.gitignore` — ignores local persistent data files.
- `docker-compose.yml` — registry default (`zhironghuang/cli-proxy-api`).
- `docker-build.sh`, `docker-build.ps1` — local image tag.
- `config.example.yaml` — fork panel repository, SQLite usage settings, Prompt Rules, Model Routes, and provider header defaults.
- `internal/config/{config,parse,sdk_config,model_routes*}.go` — fork panel defaults, SQLite usage settings, Prompt Rules and Model Routes snapshots, and provider header compatibility.
- `go.mod`, `go.sum` — fork-only SQLite/goose dependencies for persistent usage history.
- `internal/managementasset/{updater,fork_provider}.go`, `internal/managementasset/updater_release_url_test.go` — fork release feed, fallback asset wiring, and explicit prerelease release URL override for test images.
- `internal/api/handlers/management/config_basic.go` — `latestReleaseURL` (the `/v0/management/latest-version` endpoint).
- `internal/usagestore/**`, `internal/usagepersist/plugin*.go`, `internal/logging/sqlite_request_logger.go` — persistent SQLite raw events, 15-minute rollups, retained dedup keys, and request histories.
- `internal/logging/{async_emitter,request_logger}.go` and focused tests — asynchronous file logging with lossless forced-error writes and explicit flush/close behavior.
- `internal/config/prompt_rules*.go`, `internal/runtime/executor/helps/prompt_rules*.go`, `internal/api/handlers/management/prompt_rules*.go` — Prompt Rules validation, protocol rewriting, and management API.
- `internal/api/handlers/management/model_routes*.go` — Model Routes management API.
- `internal/api/server.go`, `internal/cmd/run.go`, `internal/api/handlers/management/{handler,logs,usage,prompt_rules,model_routes,config_basic}.go`, `internal/api/handlers/management/usage_test.go` — wires SQLite usage, Prompt Rules, and Model Routes into the server and management API.
- `sdk/api/handlers/{handlers,model_execution,configured_model_routes*,model_route_models}.go` and provider-specific model list handlers — applies Model Routes after plugin routing, exposes route aliases in `/models`, and preserves requested aliases in responses.
- `sdk/cliproxy/auth/response_model_rewriter.go` — response model rewriting shared by auth aliases and Model Routes.
- `internal/tui/{app,client,dashboard,i18n,usage_tab}.go` — persisted Usage view in the terminal UI.
- `internal/runtime/executor/{gemini,gemini_vertex,openai_compat,kimi,antigravity}_executor.go` and focused tests — provider header defaults; the legacy `gemini-cli-header-defaults` key is read only as a compatibility fallback for `gemini-header-defaults`.
- `sdk/cliproxy/{builder,service}.go`, `sdk/cliproxy/service_config_race_test.go` — immutable config ownership and stable snapshots for concurrent model registration during hot reload.
- `sdk/cliproxy/usage/manager.go`, `sdk/cliproxy/usage/manager_test.go` — graceful queue draining before the SQLite usage store closes.
- `sdk/cliproxy/auth/conductor.go`, `sdk/cliproxy/auth/conductor_refresh_backoff_test.go` — exponential transient refresh backoff layered onto upstream unauthorized-refresh behavior.
- `internal/api/server_test.go`, `internal/usage/logger_plugin.go`, `sdk/logging/request_logger.go`, `examples/custom-provider/main.go` — compatibility adjustments for SQLite-only request histories.
- `README.md`, `README_CN.md`, `README_JA.md` — fork notice block at the top; sponsor block replaced with an upstream pointer.
- `AGENTS.md`, `CLAUDE.md`, `docs/ai-assistant-guidance.md` — shared assistant guidance and imports.
- `.github/workflows/` — the fork keeps **no** GHA workflows. If upstream re-introduces any, re-delete them on sync.
- `.github/FUNDING.yml` — absent on the fork; do not re-add.

If you intentionally add a new customization, also add the file here so future syncs know to expect a conflict there.

## Releasing

The release tag is `zmh-vX.Y.Z` (the `zmh-` prefix avoids colliding with upstream's `vX.Y.Z` tag namespace).

1. Make sure `main` is fast-forwarded from `dev`.
2. From the local machine, log into Docker Hub as `zhironghuang` with a write-scope PAT.
3. Bake-and-push:
   ```bash
   git checkout main && git pull --ff-only origin main
   VERSION=zmh-v0.X.Y
   COMMIT=$(git rev-parse --short HEAD)
   BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
   docker buildx build \
     --platform linux/amd64,linux/arm64 \
     --build-arg VERSION="$VERSION" \
     --build-arg COMMIT="$COMMIT" \
     --build-arg BUILD_DATE="$BUILD_DATE" \
     --tag zhironghuang/cli-proxy-api:0.X.Y \
     --tag zhironghuang/cli-proxy-api:latest \
     --push .
   git tag $VERSION
   git push origin $VERSION
   ```
4. The frontend's matching `zmh-vX.Y.Z` tag (in [Cli-Proxy-API-Management-Center](https://github.com/Z-M-Huang/Cli-Proxy-API-Management-Center)) triggers its `release.yml` workflow on the self-hosted runner, publishing `management.html` as a GitHub Release asset. The backend's auto-updater fetches that asset on startup.

## Things this fork deliberately does NOT do

- We don't open PRs against `router-for-me/CLIProxyAPI` or `router-for-me/Cli-Proxy-API-Management-Center`.
- We don't add GHA workflows that depend on `DOCKERHUB_USERNAME` / `DOCKERHUB_TOKEN` secrets — backend pushes from the local machine.
- We don't carry upstream's sponsorship pointers (FUNDING.yml or README sponsor blocks). The README links back to upstream for those.
- We don't use the bare `vX.Y.Z` tag namespace — always `zmh-vX.Y.Z`.
- We don't bake `static/management.html` into the docker image. The auto-updater fetches it from our GitHub Release at startup; if the release is missing, `/management.html` 404s until the next tick.

## Fork boundary guard

[`docs/ai-assistant-guidance.md`](./docs/ai-assistant-guidance.md) documents fork-only files, patched upstream files, and the `// FORK[topic]: reason` marker convention. `scripts/check_fork_boundary.sh` enforces that list for fork-only work.

Run the boundary check before merging fork-only work:

```bash
bash scripts/check_fork_boundary.sh
```

To enable the local pre-commit hook in this clone:

```bash
git config core.hooksPath .githooks
```

## Pointers

- Upstream: <https://github.com/router-for-me/CLIProxyAPI>.
- Frontend fork: <https://github.com/Z-M-Huang/Cli-Proxy-API-Management-Center> (local sibling checkout: `../Cli-Proxy-API-Management-Center`).
