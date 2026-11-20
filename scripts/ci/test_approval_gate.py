#!/usr/bin/env python3
"""Self-tests for the test-approval gate.

These build temporary repositories. They do not approve the course work
and they do not write records into the project tree.
"""
from __future__ import annotations

import os
import subprocess
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path

SCRIPT = Path(__file__).resolve().parent / "test-approval.py"
GIT_ENV = {
    "GIT_AUTHOR_NAME": "Test",
    "GIT_AUTHOR_EMAIL": "test@example.com",
    "GIT_COMMITTER_NAME": "Test",
    "GIT_COMMITTER_EMAIL": "test@example.com",
}


def git(repo: Path, *args: str) -> str:
    env = os.environ.copy()
    env.update(GIT_ENV)
    result = subprocess.run(
        ["git", "-C", str(repo), *args],
        check=True,
        text=True,
        capture_output=True,
        env=env,
    )
    return result.stdout.strip()


def init_repo() -> Path:
    repo = Path(tempfile.mkdtemp(prefix="approval-gate-"))
    subprocess.run(["git", "init", "-q", "-b", "main", str(repo)], check=True)
    git(repo, "config", "user.email", "test@example.com")
    git(repo, "config", "user.name", "Test")
    git(repo, "config", "commit.gpgsign", "false")
    return repo


def write(repo: Path, rel: str, text: str) -> None:
    path = repo / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text)


def commit_all(repo: Path, message: str) -> str:
    git(repo, "add", "-A")
    git(repo, "commit", "-q", "-m", message)
    return git(repo, "rev-parse", "HEAD")


def approval_text(
    *,
    requirement_ids: list[str],
    plan: str,
    commit: str,
    reviewer: str = "Owner D",
    reviewer_github: str = "owner-d",
    owner: str = "Owner E",
    owner_github: str = "owner-e",
    tests: list[str] | None = None,
) -> str:
    lines = ["requirement_ids:"]
    lines.extend(f"  - {item}" for item in requirement_ids)
    lines.append(f"plan: {plan}")
    if tests:
        lines.append("tests:")
        lines.extend(f"  - {item}" for item in tests)
    lines.extend(
        [
            f"owner: {owner}",
            f"owner_github: {owner_github}",
            "decision: APPROVED",
            f"approved_commit: {commit}",
            f"reviewer: {reviewer}",
            f"reviewer_github: {reviewer_github}",
        ]
    )
    return "\n".join(lines) + "\n"


def run_gate(repo: Path, base: str) -> subprocess.CompletedProcess[str]:
    env = os.environ.copy()
    env.update(GIT_ENV)
    env["GITHUB_BASE_SHA"] = base
    env.pop("GITHUB_EVENT_PATH", None)
    env.pop("GITHUB_ACTIONS", None)
    return subprocess.run(
        [sys.executable, str(SCRIPT)],
        cwd=repo,
        env=env,
        text=True,
        capture_output=True,
    )


class TestApprovalGate(unittest.TestCase):
    def test_bound_approval_of_ancestor_with_plan_passes(self) -> None:
        repo = init_repo()
        write(repo, "docs/plans/plan.md", "plan\n")
        plan_commit = commit_all(repo, "plan")
        write(repo, "src/internal/ai/chat.go", "package ai\n")
        write(
            repo,
            "docs/evidence/test-approvals/ok.yaml",
            approval_text(requirement_ids=["FR-05"], plan="docs/plans/plan.md", commit=plan_commit),
        )
        commit_all(repo, "implement")
        result = run_gate(repo, plan_commit)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_stale_unrelated_approval_fails(self) -> None:
        repo = init_repo()
        write(repo, "docs/plans/old.md", "old plan\n")
        plan_commit = commit_all(repo, "old plan")
        write(
            repo,
            "docs/evidence/test-approvals/old.yaml",
            approval_text(requirement_ids=["FR-01"], plan="docs/plans/old.md", commit=plan_commit),
        )
        base = commit_all(repo, "historical approval")
        write(repo, "src/internal/ai/chat.go", "package ai\n\nfunc Chat() {}\n")
        commit_all(repo, "unrelated implementation")
        result = run_gate(repo, base)
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("do not intersect", result.stdout + result.stderr)

    def test_self_approval_fails(self) -> None:
        repo = init_repo()
        write(repo, "docs/plans/plan.md", "plan\n")
        plan_commit = commit_all(repo, "plan")
        write(repo, "src/internal/ai/chat.go", "package ai\n")
        write(
            repo,
            "docs/evidence/test-approvals/self.yaml",
            approval_text(
                requirement_ids=["FR-05"],
                plan="docs/plans/plan.md",
                commit=plan_commit,
                reviewer="Owner E",
                reviewer_github="owner-e",
                owner="Owner E",
                owner_github="owner-e",
            ),
        )
        commit_all(repo, "self approved implementation")
        result = run_gate(repo, plan_commit)
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("self-approval", result.stdout + result.stderr)

    def test_non_ancestor_approved_commit_fails(self) -> None:
        repo = init_repo()
        write(repo, "README.md", "base\n")
        base = commit_all(repo, "base")
        git(repo, "checkout", "-q", "-b", "side")
        write(repo, "docs/plans/plan.md", "side plan\n")
        side = commit_all(repo, "side plan")
        git(repo, "checkout", "-q", "main")
        write(repo, "src/internal/ai/chat.go", "package ai\n")
        write(
            repo,
            "docs/evidence/test-approvals/side.yaml",
            approval_text(requirement_ids=["FR-05"], plan="docs/plans/plan.md", commit=side),
        )
        commit_all(repo, "implementation pointing at a side commit")
        result = run_gate(repo, base)
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("not an ancestor", result.stdout + result.stderr)

    def test_ancestor_missing_plan_file_fails(self) -> None:
        repo = init_repo()
        write(repo, "README.md", "base\n")
        base = commit_all(repo, "base without the plan")
        write(repo, "docs/plans/plan.md", "plan added later\n")
        write(repo, "src/internal/ai/chat.go", "package ai\n")
        write(
            repo,
            "docs/evidence/test-approvals/late.yaml",
            approval_text(requirement_ids=["FR-05"], plan="docs/plans/plan.md", commit=base),
        )
        commit_all(repo, "approval of a commit that lacks the plan")
        result = run_gate(repo, base)
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("does not contain", result.stdout + result.stderr)

    def test_requirement_scope_intersection_passes_without_diff_edit(self) -> None:
        repo = init_repo()
        write(repo, "docs/plans/plan.md", "plan\n")
        plan_commit = commit_all(repo, "plan")
        write(
            repo,
            "docs/evidence/test-approvals/scoped.yaml",
            approval_text(requirement_ids=["FR-05"], plan="docs/plans/plan.md", commit=plan_commit),
        )
        base = commit_all(repo, "approval already on the base")
        write(repo, "src/internal/ai/chat.go", textwrap.dedent("""\
            package ai

            // FR-05 freshness stamp.
            func Chat() {}
            """))
        commit_all(repo, "change inside the approved requirement")
        result = run_gate(repo, base)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
