# PR 12 review remediation

Date: 2026-11-25 (Asia/Hong_Kong). Review base: `36c9beb`.
Status: implementation fixes and current local NFR measurement completed; independent acceptance remains pending.

## Implemented findings

- F-03: the partial comparison browser journey asserts the actual nonempty fixture answer, the missing-source reason, the ready-source quotation and exactly one verified navigation action. It no longer expects the removed answer-suppression message.
- F-06: the decoder rejects clear multiple-sentence output using Chinese and English terminal punctuation, in addition to the existing single-line/600-character checks. Tests cover Chinese/English negative cases, decimal numbers, abbreviations, domain names and closing quotation marks. This is a surface-format check, not semantic verification or a perfect natural-language sentence parser. No answer is cleared because a citation fails.
- F-07: the requirement documents the explicit complete-field header vocabulary and safe limited-search fallback; a new unrecognized-header case exercises the fallback. No broad completeness inference was added.
- F-08: prompt, schema and decoder now consistently use a 600-character upper bound.
- F-09: contribution 014 explicitly includes the two transcription preparation fixes and their integration purpose.

## Historical NFR evidence publication (F-05)

[Historical evidence archive](d-nfr-full-20261125-002.zip) contains the retained attempt's 120 original HTTP responses, original source snapshots, frozen manifests/cases, 120 score rows, rationales, reports and regression records. SHA-256:
`1c6ddec05c28150a83996e31114c3c93156b8076bf66f1ffdc8f6f71d6ca4c52`.

The archive manifest records original and archived hashes for every file. Only local checkout paths in three regression logs are redacted. Their historical report hashes continue to refer to the original unredacted logs; the archive explicitly records this difference. Responses, snapshots, cases, freezes and score sheets are byte-for-byte unchanged. Original local files have not been overwritten.

Run `python3 test/local/d-comparison/verify-retained.py` from the repository root to verify archive integrity, response hashes, score rows, reported totals and the recorded implementation hash against commit `99c38dd`, without a service or credentials. This reproduces the arithmetic of the recorded assisted grades, not independent semantic grading. The bundle describes revision `99c38dd` and its recorded implementation freeze. The later [PR12 full evaluation](d-nfr-pr12-20261125-001.json) independently measures the current decoder/prompt and actual saved model; the historical PASS is not transferred to those changes.

## Actual verification

- `GOCACHE=<temporary-cache> sh test/offline/d-comparison/run.sh`: PASS (comparison Go packages and 15 frontend tests).
- `python3 test/local/d-comparison/verify-retained.py`: PASS, 120 responses and 120 score rows verified.
- Both requested Playwright files were executed with one worker after downloading the matching Chromium. Result: 1 API-only test passed; 5 browser tests failed during Chromium launch, including retries. macOS reports `bootstrap_check_in ... MachPortRendezvousServer ... Permission denied (1100)` before any UI assertion. This is blocked browser verification, not a browser pass. Sanitized execution output is attached in `d-pr12-browser-result.json`.
- Layering regression and all 16 Python evaluation-tool tests: PASS.
- `make test-unit`: PASS, including 70 frontend test files / 304 tests, the production frontend build and all Go tests. `make check-race`: PASS, including full required vet and internal race checks. Initial module-cache permission failures and final passing outputs are retained separately in [validation logs](d-pr12-validation/manifest.json). All three implementation freezes still match after those checks.
- Current real-provider evaluation: 120 calls through saved `openai/gpt-5.4`, settings revision 8. Article/article answer/source scores are 18/19 then 16/16; video/video 19/20 then 18/18; video/article 17/17 then 17/17, each out of 20. Every threshold passes; the second article round is exactly at the gate, not evidence of output stability.
- All 9 HTTP format errors and 7 additional semantic failure cases are retained. The three controlled regressions pass; all 619 evidence reopens pass. Reported usage is 1,153,451 tokens. This is assisted self-evaluation, not fresh independent human grading.
- The [new published archive](d-nfr-pr12-20261125-001.zip) includes all 120 responses, scoring rationales, frozen originals/configuration and exact frozen implementation bytes. Run `python3 test/local/d-comparison/verify-public-evaluation.py --archive docs/evidence/d-nfr-pr12-20261125-001.zip` to verify it without a running service.
- Original source/track bytes and revisions were checked before and after collection. The isolated service is stopped and its temporary data/credential copy removed; no owner settings were changed.

## Missing historical evidence (F-01)

Deletion of earlier attempts remains disclosed in D-AI-030. The first complete attempt failed the video/article second-round answer threshold (15/20); a later attempt passing does not remove that failure or satisfy retention of every attempt.

The known old evaluation directories were untracked when deleted. Inspection of the two available temporary NFR backup directories found requirement/plan/document copies only, not the deleted original response sets. The current session export preserves the audit history but does not reconstruct all deleted response bytes. Full historical restoration has not been established. No waiver is signed or implied here.

An independent reviewer must either confirm authentic restoration/equivalence or explicitly decide whether to accept the documented retention exception. Decision, reviewer identity, date and scope: **pending**. Existing retention requirements remain in force for future attempts.

## Requirement ratification (F-02)

The implementation owner proposes question-focused natural-language answers and local three-profile assisted self-evaluation because these match the saved-text comparison workflow and can be executed on the owner's machine. Removing the pinned upstream baseline changes what the evaluation demonstrates: it measures absolute local thresholds, not superiority or non-regression relative to that baseline. Assisted owner grading does not provide independent scoring. These are limitations to approve or reject, not implied equivalence claims.

FR-04/NFR-04 approval fields remain pending. Independent group/instructor ratification, reviewer identity, date and decision: **pending**. No approval is inferred from test results.

## Transcript provenance (F-04)

The original session JSONL was located and frozen into a private local compressed export. [Provenance metadata](d-codex-session-provenance.json) records its exact byte count and hashes. All 179 observed turn-context records inspected at recovery identify `gpt-6.1-sol`; this runtime model label is not an immutable provider build ID. D log model labels are corrected using that evidence, retaining the build-ID limitation.

The raw export contains credentials and embedded media and is intentionally not committed. A sanitized chronological export and index of all 186 actual user messages are now published in [the session evidence directory](d-session-export/README.md). Its 10,935 records retain original ordinals and line hashes; private policy/runtime state, encrypted reasoning and media are explicitly replaced by hash tombstones. Automated redaction checks pass; owner review of the annotation presentation and course acceptance remain pending. The public projection is explicitly not byte-identical to the retained private raw export, and does not by itself establish course acceptance of the raw annotated transcript requirement. No Codex session is relabeled as OpenCode.
