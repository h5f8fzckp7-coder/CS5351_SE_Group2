#!/usr/bin/env bash
# Shared helpers for course CI gates. Intended to run from the repository root.
set -euo pipefail

repo_root() {
  git rev-parse --show-toplevel
}

cd "$(repo_root)"

base_ref() {
  if [[ -n "${GITHUB_BASE_SHA:-}" ]]; then
    echo "${GITHUB_BASE_SHA}"
    return
  fi
  if [[ -n "${GITHUB_EVENT_PATH:-}" ]] && command -v python3 >/dev/null; then
    local sha
    sha="$(python3 -c 'import json,os,sys; p=os.environ.get("GITHUB_EVENT_PATH");
import pathlib
d=json.loads(pathlib.Path(p).read_text()) if p else {};
print((d.get("pull_request") or {}).get("base",{}).get("sha") or "")' 2>/dev/null || true)"
    if [[ -n "${sha}" ]]; then
      echo "${sha}"
      return
    fi
  fi
  if git rev-parse --verify origin/main >/dev/null 2>&1; then
    echo "origin/main"
    return
  fi
  if git rev-parse --verify main >/dev/null 2>&1; then
    echo "main"
    return
  fi
  echo "HEAD"
}

changed_files() {
  local base
  base="$(base_ref)"
  if git merge-base --is-ancestor "$base" HEAD 2>/dev/null; then
    git diff --name-only "$base"...HEAD
  else
    git diff --name-only "$base" HEAD
  fi
}

is_implementation_path() {
  local f="$1"
  case "$f" in
    src/*_test.go|src/*/*_test.go|src/*/*/*_test.go|src/*/*/*/*_test.go|src/*/*/*/*/*_test.go) return 1 ;;
    src/web/src/*.test.ts|src/web/src/*.test.tsx|src/web/src/*/*.test.ts|src/web/src/*/*/*.test.ts|src/web/src/*/*/*/*.test.ts) return 1 ;;
    src/web/src/*.test.tsx|src/web/src/*/*.test.tsx|src/web/src/*/*/*.test.tsx|src/web/src/*/*/*/*.test.tsx) return 1 ;;
    src/web/src/*) return 0 ;;
    src/cmd/*|src/internal/*) return 0 ;;
    prompts/*) return 0 ;;
    *) return 1 ;;
  esac
}

is_prompt_or_model_path() {
  local f="$1"
  case "$f" in
    src/internal/ai/prompts.go|src/web/src/features/ai/promptTemplates.ts|prompts/*|test/offline/eval-b/eval-configs.yaml)
      return 0
      ;;
    *) return 1 ;;
  esac
}
