# Fork Boundary

This repo is currently being split into two maintenance surfaces:

- `refactor/upstream-bound`: shared-file changes that may become upstream PRs later.
- `refactor/fork-only`: fork-permanent behavior, governance, release wiring, and local UX.

This file governs the `refactor/fork-only` side. Treat it as the source of truth for changes that should remain local to the fork after the upstream-bound work is merged in.

## Marker convention

Use `// FORK[topic]: reason` in Go and `/* FORK[topic]: reason */` in TS/SCSS when a patch is expected to stay fork-only inside an upstream-owned file.

## Fork-owned files

These files exist only for the fork and can change freely:

- `FORK_BOUNDARY.md`
- `.githooks/pre-commit`
- `scripts/check_fork_boundary.sh`
- `internal/config/prompt_rules.go`
- `internal/config/prompt_rules_test.go`
- `internal/api/handlers/management/prompt_rules.go`
- `internal/api/handlers/management/prompt_rules_test.go`
- `internal/managementasset/fork_provider.go`
- `internal/runtime/executor/helps/prompt_rules.go`
- `internal/runtime/executor/helps/prompt_rules_test.go`
- `README.md`, `README_CN.md`, `README_JA.md`
- `CONTRIBUTING.md`

## Patched upstream files

These files may carry small fork-only deltas, but the patch must stay narrow and obvious:

- `cmd/server/main.go`
- `config.example.yaml`
- `internal/api/handlers/management/config_basic.go`
- `internal/config/config.go`
- `internal/managementasset/updater.go`
- `internal/tui/app.go`

Rule: keep fork-only edits in these files below roughly 50 changed lines per topic. If a patch wants to grow beyond that, extract it into a new fork-owned file or hook.

Exception: the one-time `prompt-rules` extraction in `internal/config/config.go` is allowed to exceed the soft limit because it removes fork-owned code from an upstream-owned file and pushes it into `internal/config/prompt_rules.go`.

## Hard-fork triggers

Stop and rethink the shape of the change if any of these happen:

- A fork-only feature needs broad edits in `internal/translator/**`.
- A single fork-only topic wants more than one upstream-owned package.
- The same fork-only patch must be re-resolved in three or more upstream syncs.
- A future upstream PR would need to carry fork branding, release tags, or repo URLs.

## Merge protocol

1. Merge or cherry-pick shared work into `refactor/upstream-bound` first.
2. Merge `refactor/upstream-bound` into `refactor/fork-only`.
3. Add fork-only commits after that merge point.
4. Run `bash scripts/check_fork_boundary.sh` before merging back into `dev`.

## Current fork topics

- `prompt-rules`: fork-owned prompt-rule type/validation cluster extracted from `internal/config/config.go`.
- `management-asset`: fork release feed and fallback asset wiring for `management.html`.
- `releases`: `zmh-v*` tag namespace and fork repo links.
