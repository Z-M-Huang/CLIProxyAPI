#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

mode="range"
range=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --staged)
      mode="staged"
      shift
      ;;
    --range)
      range="${2:-}"
      if [[ -z "$range" ]]; then
        echo "check_fork_boundary: --range requires a revision range" >&2
        exit 2
      fi
      shift 2
      ;;
    *)
      echo "check_fork_boundary: unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

owned_paths=(
  ".githooks/pre-commit"
  "AGENTS.md"
  "CLAUDE.md"
  "CONTRIBUTING.md"
  "README.md"
  "README_CN.md"
  "README_JA.md"
  "docs/ai-assistant-guidance.md"
  "internal/api/handlers/management/model_routes.go"
  "internal/api/handlers/management/model_routes_test.go"
  "internal/api/handlers/management/prompt_rules.go"
  "internal/api/handlers/management/prompt_rules_test.go"
  "internal/logging/sqlite_request_logger.go"
  "internal/logging/async_emitter.go"
  "internal/logging/async_emitter_test.go"
  "internal/logging/request_logger_bench_test.go"
  "internal/config/model_routes.go"
  "internal/config/model_routes_test.go"
  "internal/config/prompt_rules.go"
  "internal/config/prompt_rules_test.go"
  "internal/managementasset/fork_provider.go"
  "internal/managementasset/updater_release_url_test.go"
  "sdk/api/handlers/configured_model_routes.go"
  "sdk/api/handlers/configured_model_routes_test.go"
  "sdk/api/handlers/model_route_models.go"
  "internal/cache/signature_cache_bench_test.go"
  "internal/cache/signature_cache_semantics_test.go"
  "internal/runtime/executor/codex_executor_stream_chunkboundary_test.go"
  "internal/tui/usage_tab.go"
  "internal/tui/usage_tab_test.go"
  "internal/usage/logger_plugin.go"
  "internal/usage/logger_plugin_test.go"
  "internal/usagepersist/plugin.go"
  "internal/usagepersist/plugin_test.go"
  "internal/usagestore/migrations.go"
  "internal/usagestore/migrations/00001_create_usage_store.sql"
  "internal/usagestore/migrations/00002_add_cache_token_breakdown.sql"
  "internal/usagestore/migrations/00003_create_usage_rollups.sql"
  "internal/usagestore/migrations/00004_add_requested_model.sql"
  "internal/usagestore/store.go"
  "internal/usagestore/store_test.go"
  "internal/runtime/executor/helps/prompt_rules.go"
  "internal/runtime/executor/helps/prompt_rules_claude.go"
  "internal/runtime/executor/helps/prompt_rules_gemini.go"
  "internal/runtime/executor/helps/prompt_rules_interactions.go"
  "internal/runtime/executor/helps/prompt_rules_openai.go"
  "internal/runtime/executor/helps/prompt_rules_responses.go"
  "internal/runtime/executor/helps/prompt_rules_test.go"
  "sdk/cliproxy/auth/conductor_refresh_backoff_test.go"
  "sdk/cliproxy/service_config_race_test.go"
  "sdk/cliproxy/usage/manager_test.go"
  "scripts/check_fork_boundary.sh"
)

patched_upstream_paths=(
  ".gitignore"
  "Dockerfile"
  "cmd/server/main.go"
  "config.example.yaml"
  "docker-compose.yml"
  "docker-build.ps1"
  "docker-build.sh"
  "examples/plugin/claude-web-search-router/go/go.mod"
  "examples/plugin/claude-web-search-router/go/go.sum"
  "examples/custom-provider/main.go"
  "go.mod"
  "go.sum"
  "internal/api/handlers/management/handler.go"
  "internal/api/handlers/management/config_basic.go"
  "internal/api/handlers/management/logs.go"
  "internal/api/handlers/management/usage.go"
  "internal/api/handlers/management/usage_test.go"
  "internal/api/server.go"
  "internal/api/server_test.go"
  "internal/cmd/run.go"
  "internal/config/config.go"
  "internal/config/parse.go"
  "internal/config/sdk_config.go"
  "internal/config/codex_websocket_header_defaults_test.go"
  "internal/cache/signature_cache.go"
  "internal/logging/request_logger.go"
  "internal/managementasset/updater.go"
  "internal/redisqueue/queue.go"
  "internal/runtime/executor/antigravity_executor.go"
  "internal/runtime/executor/antigravity_executor_buildrequest_test.go"
  "internal/runtime/executor/gemini_executor.go"
  "internal/runtime/executor/gemini_executor_test.go"
  "internal/runtime/executor/gemini_vertex_executor.go"
  "internal/runtime/executor/kimi_executor.go"
  "internal/runtime/executor/kimi_executor_test.go"
  "internal/runtime/executor/openai_compat_executor.go"
  "internal/runtime/executor/openai_compat_executor_compact_test.go"
  "internal/tui/app.go"
  "internal/tui/client.go"
  "internal/tui/dashboard.go"
  "internal/tui/i18n.go"
  "internal/watcher/dispatcher.go"
  "sdk/api/handlers/claude/code_handlers.go"
  "sdk/api/handlers/gemini/gemini_handlers.go"
  "sdk/api/handlers/handlers.go"
  "sdk/api/handlers/model_execution.go"
  "sdk/api/handlers/openai/openai_handlers.go"
  "sdk/api/handlers/openai/openai_responses_handlers.go"
  "sdk/cliproxy/auth/response_model_rewriter.go"
  "sdk/cliproxy/service.go"
  "sdk/cliproxy/builder.go"
  "sdk/cliproxy/usage/manager.go"
  "sdk/cliproxy/auth/conductor.go"
  "sdk/config/config.go"
  "sdk/logging/request_logger.go"
)

patched_line_limit=50

patched_line_limit_override() {
  case "$1" in
    go.sum)
      echo 200
      ;;
    internal/api/handlers/management/logs.go)
      echo 250
      ;;
    internal/api/handlers/management/usage.go)
      echo 300
      ;;
    internal/api/server_test.go)
      echo 150
      ;;
    internal/cmd/run.go)
      echo 150
      ;;
    internal/config/config.go)
      echo 300
      ;;
    *)
      echo "$patched_line_limit"
      ;;
  esac
}

matches_exact_list() {
  local needle="$1"
  shift
  local item
  for item in "$@"; do
    if [[ "$needle" == "$item" ]]; then
      return 0
    fi
  done
  return 1
}

changed_files_cmd=()
numstat_cmd=()

if [[ "$mode" == "staged" ]]; then
  changed_files_cmd=(git diff --cached --name-only --diff-filter=ACMR)
  numstat_cmd=(git diff --cached --numstat --)
else
  if [[ -z "$range" ]]; then
    merge_anchor="$(git log --grep="Merge branch 'refactor/upstream-bound' into refactor/fork-only" --format=%H -n 1 || true)"
    if [[ -n "$merge_anchor" ]]; then
      range="${merge_anchor}..HEAD"
    else
      range="dev...HEAD"
    fi
  fi
  changed_files_cmd=(git diff --name-only --diff-filter=ACMR "$range")
  numstat_cmd=(git diff --numstat "$range" --)
fi

mapfile -t changed_files < <("${changed_files_cmd[@]}")

if [[ "${#changed_files[@]}" -eq 0 ]]; then
  echo "fork boundary: PASS (no changes to check)"
  exit 0
fi

violations=()

for file in "${changed_files[@]}"; do
  if matches_exact_list "$file" "${owned_paths[@]}"; then
    continue
  fi

  if matches_exact_list "$file" "${patched_upstream_paths[@]}"; then
    numstat_line="$("${numstat_cmd[@]}" "$file" | tail -n 1)"
    if [[ -n "$numstat_line" ]]; then
      IFS=$'\t' read -r added deleted _ <<<"$numstat_line"
      if [[ "$added" != "-" && "$deleted" != "-" ]]; then
        file_limit="$(patched_line_limit_override "$file")"
        changed_lines=$((added + deleted))
        if (( changed_lines > file_limit )); then
          violations+=("patched upstream file exceeds ${file_limit} lines: ${file} (${changed_lines})")
        fi
      fi
    fi
    continue
  fi

  violations+=("fork-only change escaped customization surface: ${file}")
done

if [[ "${#violations[@]}" -gt 0 ]]; then
  printf 'fork boundary: FAIL\n' >&2
  printf '  - %s\n' "${violations[@]}" >&2
  exit 1
fi

echo "fork boundary: PASS"
