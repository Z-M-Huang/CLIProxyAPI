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
  "internal/api/handlers/management/prompt_rules.go"
  "internal/api/handlers/management/prompt_rules_test.go"
  "internal/logging/sqlite_request_logger.go"
  "internal/config/prompt_rules.go"
  "internal/config/prompt_rules_test.go"
  "internal/managementasset/fork_provider.go"
  "internal/usagepersist/plugin.go"
  "internal/usagestore/migrations.go"
  "internal/usagestore/migrations/00001_create_usage_store.sql"
  "internal/usagestore/store.go"
  "internal/usagestore/store_test.go"
  "internal/runtime/executor/helps/prompt_rules.go"
  "internal/runtime/executor/helps/prompt_rules_test.go"
  "scripts/check_fork_boundary.sh"
)

patched_upstream_paths=(
  ".gitignore"
  "cmd/server/main.go"
  "config.example.yaml"
  "docker-compose.yml"
  "examples/custom-provider/main.go"
  "go.mod"
  "go.sum"
  "internal/api/handlers/management/handler.go"
  "internal/api/handlers/management/config_basic.go"
  "internal/api/handlers/management/logs.go"
  "internal/api/handlers/management/usage.go"
  "internal/api/server.go"
  "internal/api/server_test.go"
  "internal/cmd/run.go"
  "internal/config/config.go"
  "internal/managementasset/updater.go"
  "internal/usage/logger_plugin.go"
  "internal/tui/app.go"
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
