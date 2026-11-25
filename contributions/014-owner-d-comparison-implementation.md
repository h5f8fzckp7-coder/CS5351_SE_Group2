# 014 · Implement owner D multi-content comparison

- **Task owner:** Wenlong Lu (D).
- **Branch:** `owner/d-multi-content-comparison`.
- **Implementation commit:** `99c38dd`; follow-up addresses review findings on PR 12.
- **AI tool:** Codex; recovered turn-context model label `gpt-6.1-sol`; immutable provider build identifier unavailable.
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

A private original Codex session export is retained locally; a sanitized chronological projection and actual-user-intervention index are published in [session evidence](../docs/evidence/d-session-export/README.md). Original and public-artifact hashes are recorded; media/private runtime content is explicitly redacted. No course acceptance or independent signature is claimed.

## Transcription integration fixes

This contribution also includes the Bilibili signed-audio probe Range header and a single CPU retry after a confirmed whisper.cpp Metal allocation failure, with targeted adapter tests; these support original subtitle preparation before comparison.

## PR review follow-up

See [review remediation](../docs/evidence/d-pr12-remediation.md) for current fixes, published historical evidence, actual test results and unresolved approvals. The historical retained pass describes the pre-review implementation. The [new complete local evaluation](../docs/evidence/d-nfr-pr12-20261125-001.json) separately measures the current implementation and saved `openai/gpt-5.4` model: all six rounds meet both thresholds, all three controlled regressions pass, and all 619 evidence reopens pass. All nine HTTP format failures and seven additional semantic failure cases remain archived; independent acceptance is pending.
