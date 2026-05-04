# AI Assistant Guidance

Go 1.26+ proxy server providing OpenAI/Gemini/Claude/Codex compatible APIs with OAuth and round-robin load balancing.

## Read First

This is the [Z-M-Huang/CLIProxyAPI](https://github.com/Z-M-Huang/CLIProxyAPI) soft fork of [router-for-me/CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI). Before opening any branch or PR, read [`CONTRIBUTING.md`](../CONTRIBUTING.md) for the fork-specific workflow.

This fork no longer tries to upstream its improvements. Treat upstream as an input stream only: selectively merge or cherry-pick useful changes from `upstream/dev` into our `dev`, then keep this fork's changes on top. Do not open PRs against `router-for-me/*` unless the maintainer explicitly reverses this policy.

Hot rules:

1. Cut new branches from `dev`, never `main`. Open PRs against this fork's `dev`.
2. Do not open PRs against `router-for-me/*`; upstream merge is no longer the goal.
3. Release tags are `zmh-vX.Y.Z`, never bare `vX.Y.Z` because upstream owns that namespace.
4. The backend Docker image is `zhironghuang/cli-proxy-api` and is pushed manually from a local machine.
5. This fork keeps no GitHub Actions workflows for backend Docker publishing. If upstream reintroduces workflows, delete them during sync.
6. Customization conflicts during upstream sync are expected only in the files listed in `CONTRIBUTING.md`; keep our version there and take upstream elsewhere.
7. The management frontend is released from `Z-M-Huang/Cli-Proxy-API-Management-Center`; the backend updater fetches the release asset at startup instead of baking `static/management.html` into the Docker image.

## Repositories

- Backend fork: <https://github.com/Z-M-Huang/CLIProxyAPI>
- Backend upstream: <https://github.com/router-for-me/CLIProxyAPI>
- Frontend fork: <https://github.com/Z-M-Huang/Cli-Proxy-API-Management-Center>
- Local frontend checkout: `../Cli-Proxy-API-Management-Center`

## Commands

```bash
gofmt -w . # Format after Go changes
go build -o cli-proxy-api ./cmd/server # Build
go run ./cmd/server # Run dev server
go test ./... # Run all tests
go test -v -run TestName ./path/to/pkg # Run a single test
go build -o test-output ./cmd/server && rm test-output # Required compile check after Go changes
```

Common flags: `--config <path>`, `--tui`, `--standalone`, `--local-model`, `--no-browser`, `--oauth-callback-port <port>`.

## Config

- Default config: `config.yaml` with template in `config.example.yaml`.
- `.env` is auto-loaded from the working directory.
- Auth material defaults under `auths/`.
- Storage backends: file-based default; optional Postgres, git, and object store via `PGSTORE_*`, `GITSTORE_*`, and `OBJECTSTORE_*`.

## Architecture

- `cmd/server/`: server entrypoint.
- `internal/api/`: Gin HTTP API, routes, middleware, and modules.
- `internal/api/modules/amp/`: Amp integration, Amp-style routes, and reverse proxy.
- `internal/thinking/`: main thinking/reasoning pipeline. `ApplyThinking()` in `apply.go` parses suffixes in `suffix.go`, suffix overrides body, normalizes config to canonical `ThinkingConfig` in `types.go`, normalizes and validates centrally in `validate.go` and `convert.go`, then applies provider-specific output through `ProviderApplier`. Do not break the canonical representation to per-provider translation architecture.
- `internal/runtime/executor/`: per-provider runtime executors, including Codex WebSocket.
- `internal/translator/`: provider protocol translators and shared `common`.
- `internal/registry/`: model registry and remote updater through `StartModelsUpdater`; `--local-model` disables remote updates.
- `internal/store/`: storage implementations and secret resolution.
- `internal/managementasset/`: config snapshots and management assets.
- `internal/cache/`: request signature caching.
- `internal/watcher/`: config hot reload and watchers.
- `internal/wsrelay/`: WebSocket relay sessions.
- `internal/usage/`: usage and token accounting.
- `internal/tui/`: Bubbletea terminal UI for `--tui` and `--standalone`.
- `sdk/cliproxy/`: embeddable SDK entry, service, builder, watchers, and pipeline.
- `test/`: cross-module integration tests.

## Code Conventions

- Keep changes small and simple.
- Comments in code must be in English.
- If editing code that already contains non-English comments, translate them to English.
- For user-visible strings, keep the existing language used in that file or area.
- New Markdown docs should be in English unless the file is explicitly language-specific, such as `README_CN.md`.
- Follow `gofmt`; keep imports goimports-style.
- Wrap errors with context where helpful.
- Do not use `log.Fatal` or `log.Fatalf`; prefer returning errors and logging through logrus.
- For shadowed variables, use a method suffix such as `errStart := server.Start()`.
- Wrap defer errors, for example: `defer func() { if err := f.Close(); err != nil { log.Errorf(...) } }()`.
- Use logrus structured logging.
- Do not leak secrets or tokens in logs.
- Avoid panics in HTTP handlers; prefer logged errors and meaningful HTTP status codes.

Timeout rule: timeouts are allowed only during credential acquisition. After an upstream connection is established, do not set timeouts for subsequent network behavior. Intentional exceptions are the Codex websocket liveness deadlines in `internal/runtime/executor/codex_websockets_executor.go`, wsrelay session deadlines in `internal/wsrelay/session.go`, the management APICall timeout in `internal/api/handlers/management/api_tools.go`, and utility timeouts in `cmd/fetch_antigravity_models`.

## Translator And Executor Boundaries

As a rule, do not make standalone changes to `internal/translator/`. You may modify it only as part of broader changes elsewhere.

If a task requires changing only `internal/translator/`, run:

```bash
gh repo view --json viewerPermission -q .viewerPermission
```

Proceed only if the result is `WRITE`, `MAINTAIN`, or `ADMIN`. Otherwise, file a GitHub issue with the goal, rationale, and intended implementation code, then stop further work.

`internal/runtime/executor/` should contain executors and their unit tests only. Place helper and support files under `internal/runtime/executor/helps/`.

## Fork Boundary Guard

Use `// FORK[topic]: reason` in Go and `/* FORK[topic]: reason */` in TS/SCSS when a patch is expected to stay fork-only inside an upstream-owned file.

Fork-owned files:

- `AGENTS.md`
- `CLAUDE.md`
- `CONTRIBUTING.md`
- `docs/ai-assistant-guidance.md`
- `.githooks/pre-commit`
- `scripts/check_fork_boundary.sh`
- `README.md`, `README_CN.md`, `README_JA.md`
- `internal/config/prompt_rules.go`
- `internal/config/prompt_rules_test.go`
- `internal/api/handlers/management/prompt_rules.go`
- `internal/api/handlers/management/prompt_rules_test.go`
- `internal/managementasset/fork_provider.go`
- `internal/runtime/executor/helps/prompt_rules.go`
- `internal/runtime/executor/helps/prompt_rules_test.go`

Patched upstream files:

- `cmd/server/main.go`
- `config.example.yaml`
- `internal/api/handlers/management/config_basic.go`
- `internal/config/config.go`
- `internal/managementasset/updater.go`
- `internal/tui/app.go`

Hard-fork triggers:

- A fork-only feature needs broad edits in `internal/translator/**`.
- A single fork-only topic wants more than one upstream-owned package.
- The same fork-only patch must be re-resolved in three or more upstream syncs.
- A future upstream PR would need fork branding, release tags, or repo URLs.

Run the boundary check before merging fork-only work:

```bash
bash scripts/check_fork_boundary.sh
```

To enable the local pre-commit hook in this clone:

```bash
git config core.hooksPath .githooks
```

## Upstream Sync Policy

Use upstream as a source of selected fixes and maintenance, not as a contribution target.

1. Fetch `origin` and `upstream`.
2. Inspect what changed with `git log --oneline origin/dev..upstream/dev`.
3. Create a sync branch from `origin/dev`.
4. Cherry-pick selected commits, or merge `upstream/dev` only when it is low conflict.
5. Resolve expected conflicts in the customization surface by keeping our fork version.
6. Investigate conflicts outside the customization surface before resolving them.
7. Open the sync PR against `Z-M-Huang/CLIProxyAPI` `dev`.
8. Layer fork-only changes on top of our `dev` after sync work lands, or in a separate feature branch from `dev`.

Current fork topics:

- `prompt-rules`: fork-owned prompt-rule type and validation cluster extracted from `internal/config/config.go`.
- `management-asset`: fork release feed and fallback asset wiring for `management.html`.
- `releases`: `zmh-v*` tag namespace and fork repo links.

## Pointers

- `CONTRIBUTING.md`: fork workflow, branch model, release process, and customization surface.
- Frontend release repo: `../Cli-Proxy-API-Management-Center`.
- Local plan history, when present: `/home/ubuntu/.claude/plans/we-are-in-a-nested-emerson.md`.
