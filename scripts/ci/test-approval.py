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
REQ_RE = re.compile(r"\b(?:FR|NFR|INV)-\d+\b")
LIST_KEYS = {"requirement_ids", "tests"}


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
    print(
        "test-approval: no pull-request base SHA and no main ref",
        file=sys.stderr,
    )
    raise SystemExit(1)


def ensure_commit(ref: str) -> None:
    try:
        git("rev-parse", "--verify", "--quiet", f"{ref}^{{commit}}")
        return
    except subprocess.CalledProcessError:
        pass
    if os.environ.get("GITHUB_ACTIONS") == "true":
        try:
            git("fetch", "--no-tags", "origin", ref)
            git("rev-parse", "--verify", "--quiet", f"{ref}^{{commit}}")
            return
        except subprocess.CalledProcessError:
            pass
    print(
        f"test-approval: base {ref} is not available; fetch the pull request base before running this gate",
        file=sys.stderr,
    )
    raise SystemExit(1)


def diff_names(base: str) -> str:
    try:
        git("merge-base", "--is-ancestor", base, "HEAD")
        return git("diff", "--name-only", f"{base}...HEAD")
    except subprocess.CalledProcessError:
        return git("diff", "--name-only", base, "HEAD")


def diff_text(base: str) -> str:
    try:
        git("merge-base", "--is-ancestor", base, "HEAD")
        return git("diff", "-U0", f"{base}...HEAD")
    except subprocess.CalledProcessError:
        return git("diff", "-U0", base, "HEAD")


def changed_files(base: str) -> list[str]:
    return [line for line in diff_names(base).splitlines() if line]


def changed_requirement_scope(base: str) -> set[str]:
    return set(REQ_RE.findall(diff_text(base)))


def is_implementation(path: str) -> bool:
    if TEST_RE.search(path):
        return False
    return bool(IMPL_RE.match(path))


def parse_record(path: Path) -> dict:
    data: dict = {"requirement_ids": [], "tests": []}
    current_list: str | None = None
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.split("#", 1)[0].rstrip()
        if not line:
            current_list = None
            continue
        if line.startswith((" ", "\t")):
            item = line.strip()
            if item.startswith("- ") and current_list:
                data[current_list].append(item[2:].strip().strip('"').strip("'"))
            continue
        if ":" not in line:
            current_list = None
            continue
        key, value = line.split(":", 1)
        key = key.strip()
        value = value.strip().strip('"').strip("'")
        if key in LIST_KEYS:
            if value:
                data[key] = [
                    part.strip().strip('"').strip("'")
                    for part in value.strip("[]").split(",")
                    if part.strip()
                ]
                current_list = None
            else:
                data[key] = []
                current_list = key
            continue
        current_list = None
        data[key] = value
    return data


def is_ancestor(commit: str, head: str = "HEAD") -> bool:
    try:
        git("merge-base", "--is-ancestor", commit, head)
        return True
    except subprocess.CalledProcessError:
        return False


def commit_has_path(commit: str, path: str) -> bool:
    if not path or path.startswith("/") or ".." in Path(path).parts:
        return False
    try:
        out = git("ls-tree", "-r", "--name-only", commit, "--", path)
    except subprocess.CalledProcessError:
        return False
    return path in out.splitlines()


def referenced_paths(data: dict) -> list[str]:
    paths: list[str] = []
    plan = str(data.get("plan", "")).strip()
    if plan:
        paths.append(plan)
    for item in data.get("tests") or []:
        text = str(item).strip()
        if text:
            paths.append(text)
    return paths


def approval_bound(record_rel: str, data: dict, files: list[str], scope: set[str]) -> bool:
    if record_rel in files:
        return True
    ids = {str(item).strip() for item in (data.get("requirement_ids") or []) if str(item).strip()}
    return bool(ids & scope)


def main() -> int:
    base = base_ref()
    ensure_commit(base)
    files = changed_files(base)
    implementation = [f for f in files if is_implementation(f)]
    if not implementation:
        print("test-approval: no implementation paths changed; gate passes.")
        return 0

    print("implementation paths changed:")
    for f in implementation:
        print(f"  {f}")

    scope = changed_requirement_scope(base)
    records = sorted((ROOT / "docs/evidence/test-approvals").glob("*.yaml"))
    approved = []
    for record in records:
        rel = str(record.relative_to(ROOT))
        data = parse_record(record)
        if data.get("decision") != "APPROVED":
            print(f"{rel}: decision={data.get('decision') or '(missing)'}")
            continue
        reviewer = str(data.get("reviewer", "")).strip()
        owner = str(data.get("owner", "")).strip()
        owner_gh = str(data.get("owner_github", "")).strip().lstrip("@")
        reviewer_gh = str(data.get("reviewer_github", "")).strip().lstrip("@")
        commit = str(data.get("approved_commit", "")).strip()
        if not reviewer or not commit:
            print(f"{rel}: APPROVED but missing reviewer or approved_commit")
            continue
        if reviewer == owner or (reviewer_gh and owner_gh and reviewer_gh == owner_gh):
            print(f"{rel}: reviewer matches owner; self-approval is rejected")
            continue
        if not re.fullmatch(r"[0-9a-f]{7,40}", commit):
            print(f"{rel}: approved_commit is not a git SHA")
            continue
        if not is_ancestor(commit):
            print(f"{rel}: approved_commit {commit} is not an ancestor of HEAD")
            continue
        referenced = referenced_paths(data)
        if not referenced:
            print(f"{rel}: APPROVED record names no plan or test files to verify")
            continue
        missing = [path for path in referenced if not commit_has_path(commit, path)]
        if missing:
            print(
                f"{rel}: approved_commit {commit} does not contain referenced plan/test files: "
                + ", ".join(missing)
            )
            continue
        if not approval_bound(rel, data, files, scope):
            print(
                f"{rel}: approval is not in this diff and its requirement_ids "
                "do not intersect the changed requirement scope"
            )
            continue
        approved.append(record)
        print(f"{rel}: accepted approval by {reviewer} of {commit}")

    if not approved:
        print(
            "test-approval: implementation changes require an APPROVED record that "
            "is part of this diff or whose requirement_ids intersect the changed "
            "requirement scope, with a non-author reviewer and an approved_commit "
            "that is an ancestor of HEAD and that contains the referenced plan/test "
            "files. Pending templates and unrelated historical approvals do not "
            "satisfy this gate.",
            file=sys.stderr,
        )
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
