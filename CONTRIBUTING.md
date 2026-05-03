# Contributing to Z-M-Huang/CLIProxyAPI

This is a soft fork of [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI). The fork carries fork-specific features on top of upstream while continuing to absorb upstream improvements over time.

If you're a human collaborator or an AI coding assistant, **read this before opening a branch or PR**. The same workflow is referenced by [`AGENTS.md`](./AGENTS.md) and [`CLAUDE.md`](./CLAUDE.md).

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
- **`feat/logging`** is reserved for the deferred v0.2.0 logging effort. Don't reuse the name.

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

- `Dockerfile` — currently aligned with upstream; reserved for future fork-specific bake steps.
- `docker-compose.yml` — registry default (`zhironghuang/cli-proxy-api`).
- `docker-build.sh`, `docker-build.ps1` — local image tag.
- `config.example.yaml` — `panel-github-repository` points at our fork.
- `internal/config/config.go` — `DefaultPanelGitHubRepository`.
- `internal/managementasset/updater.go` — `defaultManagementReleaseURL`; the upstream `cpamc.router-for.me` fallback is intentionally absent.
- `internal/api/handlers/management/config_basic.go` — `latestReleaseURL` (the `/v0/management/latest-version` endpoint).
- `README.md`, `README_CN.md`, `README_JA.md` — fork notice block at the top; sponsor block replaced with an upstream pointer.
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

The `feat/logging` deferred work targets the next release (`zmh-v0.2.0`).

## Things this fork deliberately does NOT do

- We don't open PRs against `router-for-me/CLIProxyAPI` or `router-for-me/Cli-Proxy-API-Management-Center`.
- We don't add GHA workflows that depend on `DOCKERHUB_USERNAME` / `DOCKERHUB_TOKEN` secrets — backend pushes from the local machine.
- We don't carry upstream's sponsorship pointers (FUNDING.yml or README sponsor blocks). The README links back to upstream for those.
- We don't use the bare `vX.Y.Z` tag namespace — always `zmh-vX.Y.Z`.
- We don't bake `static/management.html` into the docker image. The auto-updater fetches it from our GitHub Release at startup; if the release is missing, `/management.html` 404s until the next tick.

## Fork boundary guard

`FORK_BOUNDARY.md` is the source of truth for fork-only files, patched upstream files, and the `// FORK[topic]: reason` marker convention.

Run the boundary check before merging fork-only work:

```bash
bash scripts/check_fork_boundary.sh
```

To enable the local pre-commit hook in this clone:

```bash
git config core.hooksPath .githooks
```

## Pointers

- Plan / decision history: `/home/ubuntu/.claude/plans/we-are-in-a-nested-emerson.md` (local-only).
- Upstream: <https://github.com/router-for-me/CLIProxyAPI>.
- Frontend fork: <https://github.com/Z-M-Huang/Cli-Proxy-API-Management-Center>.
