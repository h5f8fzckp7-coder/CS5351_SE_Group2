#!/usr/bin/env python3
"""Fail when implementation changes lack a non-author test approval record."""
from __future__ import annotations

import os
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip())
os.chdir(ROOT)

IMPL_RE = re.compile(
    r"^(src/(cmd|internal)/.*(?<!_test)\.go|src/web/src/.*(?<!\.test)\.(ts|tsx)|prompts/.*)$"
)
TEST_RE = re.compile(r"(_test\.go|\.test\.(ts|tsx))$")


def git(*args: str) -> str:
    return subprocess.check_output(["git", *args], text=True).strip()


def base_ref() -> str:
    if os.environ.get("GITHUB_BASE_SHA"):
        return os.environ["GITHUB_BASE_SHA"]
    event = os.environ.get("GITHUB_EVENT_PATH")
    if event and Path(event).exists():
        import json

        data = json.loads(Path(event).read_text())
        sha = ((data.get("pull_request") or {}).get("base") or {}).get("sha")
        if sha:
            return sha
    for cand in ("origin/main", "main"):
        try:
            git("rev-parse", "--verify", cand)
            return cand
        except subprocess.CalledProcessError:
            continue
    return "HEAD"


def changed_files(base: str) -> list[str]:
    try:
        git("merge-base", "--is-ancestor", base, "HEAD")
        out = git("diff", "--name-only", f"{base}...HEAD")
    except subprocess.CalledProcessError:
        out = git("diff", "--name-only", base, "HEAD")
    return [line for line in out.splitlines() if line]


def is_implementation(path: str) -> bool:
    if TEST_RE.search(path):
        return False
    return bool(IMPL_RE.match(path))


def parse_simple_yaml(path: Path) -> dict[str, str]:
    data: dict[str, str] = {}
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.split("#", 1)[0].rstrip()
        if not line or line.startswith(" ") or line.startswith("-"):
            continue
        if ":" not in line:
            continue
        key, value = line.split(":", 1)
        data[key.strip()] = value.strip().strip('"').strip("'")
    return data


def is_ancestor(commit: str, head: str = "HEAD") -> bool:
    try:
        git("merge-base", "--is-ancestor", commit, head)
        return True
    except subprocess.CalledProcessError:
        return False


def main() -> int:
    base = base_ref()
    files = changed_files(base)
    implementation = [f for f in files if is_implementation(f)]
    if not implementation:
        print("test-approval: no implementation paths changed; gate passes.")
        return 0

    print("implementation paths changed:")
    for f in implementation:
        print(f"  {f}")

    records = sorted((ROOT / "docs/evidence/test-approvals").glob("*.yaml"))
    approved = []
    for record in records:
        data = parse_simple_yaml(record)
        if data.get("decision") != "APPROVED":
            print(f"{record.relative_to(ROOT)}: decision={data.get('decision') or '(missing)'}")
            continue
        reviewer = data.get("reviewer", "").strip()
        owner = data.get("owner", "").strip()
        owner_gh = data.get("owner_github", "").strip().lstrip("@")
        reviewer_gh = data.get("reviewer_github", "").strip().lstrip("@")
        commit = data.get("approved_commit", "").strip()
        if not reviewer or not commit:
            print(f"{record.relative_to(ROOT)}: APPROVED but missing reviewer or approved_commit")
            continue
        if reviewer == owner or (reviewer_gh and owner_gh and reviewer_gh == owner_gh):
            print(f"{record.relative_to(ROOT)}: reviewer matches owner; self-approval is rejected")
            continue
        if not re.fullmatch(r"[0-9a-f]{7,40}", commit):
            print(f"{record.relative_to(ROOT)}: approved_commit is not a git SHA")
            continue
        if not is_ancestor(commit):
            print(f"{record.relative_to(ROOT)}: approved_commit {commit} is not an ancestor of HEAD")
            continue
        approved.append(record)
        print(f"{record.relative_to(ROOT)}: accepted approval by {reviewer} of {commit}")

    if not approved:
        print(
            "test-approval: implementation changes require a non-author APPROVED "
            "record in docs/evidence/test-approvals/ whose approved_commit is an "
            "ancestor of HEAD. Pending templates do not satisfy this gate.",
            file=sys.stderr,
        )
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
