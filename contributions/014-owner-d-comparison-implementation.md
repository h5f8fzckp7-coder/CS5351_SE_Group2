# 014 · Implement owner D multi-content comparison

- **Task owner:** Wenlong Lu (D).
- **Branch:** `owner/d-multi-content-comparison`.
- **Commit:** included in this implementation commit; no push or PR is claimed.
- **AI tool:** Codex; exact model build identifier was not captured.
- **Scope:** FR-04 implementation, its functional tests, and integration with
  A's generation ledger, B's retrieval engine, and C's citation validation.
- **Related:** [requirements](../docs/req/requirements.yaml),
  [plan](../docs/plans/d-comparison-plan.md),
  [verification](../docs/evidence/d-fr-verification.md),
  [offline entry point](../test/offline/d-comparison/README.md).
- **Review:** implementation was explicitly requested in the Codex conversation
  on 2026-11-20 and continued on 2026-11-22. Independent acceptance and any
  missing historical test-approval evidence remain pending. No approval is
  reconstructed or signed on behalf of another member.

## Implementation

The [FR-04 specification](../docs/req/fr-04-multi-content-comparison.md)
defines two/three-source comparison of saved articles and original video subtitle
text, including mixed selections and questions focused on a subset. The UI
provides saved-source readiness, preparation links, inline original-track
selection, revision checks and readiness refresh.

The configured conversation model produces a concise natural-language sentence
with ordered per-source findings. Source/passage handles preserve attribution;
A budgets every completion attempt, B bounds retrieval/context, and C verifies
original evidence and reopening. Generated decoded answers remain visible
regardless of citation verification. Invalid or stale evidence retains an
explicit source status and loses verified navigation. Comparison does not
modify single-content chat history.

## Verification and limits

Functional tests and manual isolated browser checks are described in the linked
verification record. All fixtures and UI preparation data are synthetic.
Playwright browser launch is blocked by this machine's sandbox; this is not
reported as a passing browser test. Local-model smoke checks are not the
three-profile [local NFR-04 self-evaluation](../test/local/d-comparison/README.md) and establish no 80 percent accuracy claim. Each profile has 20 questions and two rounds. The author runs and scores that evaluation locally; independent PR review remains separate.

The [retained complete NFR evaluation](../docs/evidence/d-nfr-full-repeat-20261125-002.json)
contains 120 real-model responses across article/article, video/video and
video/article. Both metrics in every round reach 16/20, all controlled
regressions pass, and all 732 evidence reopens are valid. Scores were prepared
by Codex under the owner's standing original-text grading delegation; no fresh
independent human numerical review or PR approval is claimed. All 120 raw
responses, original snapshots, freezes, per-case rationale and residual errors
are retained. Four multi-sentence findings remain separate FR defects.

The raw Codex conversation export has not been retained as a repository artifact.
The AI-use log records that gap rather than inventing an export or hash.
