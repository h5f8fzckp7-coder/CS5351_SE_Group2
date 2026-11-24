# D · FR-04 functional verification

- **Owner:** Wenlong Lu (D).
- **Date:** 2026-11-22.
- **Branch:** `owner/d-multi-content-comparison`.
- **Status:** implementation and controlled functional checks are ready for
  independent review; no independent acceptance, merge, or NFR pass is claimed.
- **Related:** [FR-04 specification](../req/fr-04-multi-content-comparison.md),
  [requirements](../req/requirements.yaml),
  [implementation plan](../plans/d-comparison-plan.md),
  [offline entry point](../../test/offline/d-comparison/README.md).

## Implemented behavior

The local library exposes comparison of two or three saved articles or original
video subtitle sources. Unsupported saved items stay visible with a disabled
choice and explanation. The local-only readiness endpoint provides preparation
links and refresh behavior before a generation request is attempted. If no
selected source is ready the UI disables submission. The server independently
validates every request and makes zero generation calls when all sources lack
usable text. The result starts with the model's one-sentence answer instead of
a fixed category. A successfully decoded generated answer remains visible
regardless of whether any or all source checks pass. A partial failure retains
other verified findings and shows the affected source's reason. Reopening stale
evidence updates that source and its jumps without clearing the answer.

Generation uses invocation-scoped source and passage handles. The server
restores native provenance only after checking exact source membership and
uniqueness. A schema limits handles and states, but never grants authority to
model output. Foreign snapshots are rejected before generation; C validates
exact source quotations after generation and on reopening. Changed/deleted
sources lose their displayed facts and enabled verified jumps while other
sources remain available. Single-content chat history is not modified.

Short complete sources are included directly; longer inputs use B retrieval.
Schema text, the system instruction, question and source context count toward
both character and UTF-8 byte bounds. Schema/JSON/plain compatibility attempts
share A's invocation ledger; blocked starts, cancellation and unknown usage
keep distinct terminal states. B embedding work is accounted separately.

## Acceptance traceability

| Test ID | Acceptance checks | Canonical executable evidence |
| --- | --- | --- |
| D-T01 | 2/3 distinct IDs; invalid IDs/count/question/revision/limit; generic unavailable-source errors; local readiness without AI configuration | `TestComparisonValidationBeforeProviderCalls`, `TestComparisonSourceInspectionRejectsInvalidInput`, `TestComparisonSourceInspectionHTTPWithoutAIConfiguration`; frontend `comparisonSources.test.tsx` |
| D-T02 | One-sentence answer, ordered unique per-source findings and evidence states, stable attribution, original evidence and complete-field absence | `TestComparisonVerifiedConflictAndSourceNavigation`, `TestComparisonThreeSourcesRemainInInputOrder`, `TestComparisonCompleteAbsenceAndLimitedCoverage`, `TestComparisonScopedSourceHandlesAndReorderedFindings`; `ComparisonPanel.test.tsx` |
| D-T03 | Partial/all-source read failures, missing original captions, changed snapshots, oversized units; no generation when nothing is usable | `TestComparisonPartialAndAllSourceFailures`, `TestComparisonDetectsChangeDuringGeneration`, `TestComparisonOversizedSourceIsExplicitAndStartsNoGeneration`, `TestComparisonHTTPWithSavedArticleAndSelectedOriginalCaptions` |
| D-T04 | Cross-source/altered/malformed/duplicate findings, unknown handles, foreign snapshots, stale/deleted/out-of-range source access; decoded answer retained when citation checks fail | `TestComparisonRejectsWrongSourceQuoteAndMalformedOutput`, `TestComparisonUnknownSourceHandleFailsClosed`, `TestComparisonForeignSnapshotNeverReachesModel`; source reopening integration and frontend stale-source tests |
| D-T05 | A ledger across format fallbacks; inherited limits; unknown usage; cancellation; bounded long-source retrieval; schema included in context bounds | `TestComparisonStructuredOutputAndBudgetedCompatibilityFallback`, `TestComparisonBudgetUnknownCancellationAndInheritedLimit`, `TestComparisonLongSourceUsesBoundedRetrieval`, `TestComparisonStructuredContractAndContextBounds` |
| D-T07 | Real HTTP/SQLite composition; selected original caption and timestamp; unchanged chat history; preparation refresh; partial findings and stale jumps | `comparison_integration_test.go`; existing `reader-chat.spec.ts` extended with comparison journeys; isolated in-app browser checks below |
| D-T06 | Three separately frozen 20-question profiles, two complete rounds each; owner-delegated source-grounded scoring and controlled regression | **Latest full repeat PASS:** article/article 18/20 and 20/20, then 20/20 and 20/20; video/video 18/20 and 19/20, then 20/20 and 20/20; video/article 17/20 and 18/20, then 18/20 and 18/20 (answer then source). All regressions and 732 evidence reopens pass. [Latest record](d-nfr-full-repeat-20261125-002.json). Four separate sentence-format findings remain; no independent review is claimed. |

The Go service tests are in
[`comparison_test.go`](../../src/internal/ai/comparison_test.go) and
[`comparison_sources_test.go`](../../src/internal/ai/comparison_sources_test.go).
HTTP/SQLite integration is in
[`comparison_integration_test.go`](../../src/internal/app/comparison_integration_test.go).
Provider-format checks are in
[`aichat/comparison_test.go`](../../src/internal/adapters/aichat/comparison_test.go).

The current NFR-04 acceptance method is the [local author self-evaluation protocol](../../test/local/d-comparison/README.md). Dated observations below retain the procedure and limitations recorded at execution time; they do not declare completion of the current quality evaluation.

## Executed checks

### Focused questions with unrelated selected sources (2026-11-22)

The user selected three articles but asked only about the game-review article.
The earlier all-source answer gate discarded the valid answer because the
other two sources were marked insufficient. The prompt and schema now support
`not_relevant` for completely supplied, revalidated sources that the question
does not ask about. These rows have no claims or citations and display a neutral
explanation. At least one relevant verified finding is still required. Missing
text, read/range failures, stale versions, partial retrieval, invalid citations,
and attempted claims on excluded rows remain blocking failures.

Regression tests cover a focused three-source answer, all-unrelated selections,
attempts to hide missing/read-failed/stale sources, incomplete input, claims or
citations on excluded rows, schema restrictions, and range revalidation. The
AI, aichat, app and HTTP tests passed; the final comparison tests and vet passed.
Two frontend test files passed (11 tests), and the frontend production build
passed with the existing chunk-size warning. Browser E2E was not executed.

Two real-model runs used an isolated database backup and the owner's configured
`openai/gpt-5.4-mini`. The exact focused question from the screenshot returned
an answer in about 5.3 seconds, excluded the first two sources, and kept the
game-review source with two verified citations and successful exact reopens.
A control question requesting all three topics returned three supported rows
in about 4.9 seconds with ten verified citations and successful reopens. Each
run started one provider call. The GTA item still contains only a saved RSS
excerpt; this is not a verification of the unavailable full article or its claims.
See [d-comparison-relevance-smoke-20261122.json](d-comparison-relevance-smoke-20261122.json).
These are regression checks, not NFR-04 acceptance or a general accuracy score.

### One-sentence answer presentation (2026-11-22)

At the owner's explicit request, the model output contract now requires a
bounded free-text answer instead of choosing an overall relationship enum.
Source evidence still uses structured states and source-scoped citation IDs.
The server rejects empty, multiline, and oversized answers, withholds synthesis
when source verification fails, and clears the answer when reopened evidence
is stale. The UI renders the answer as escaped text under Comparison answer;
legacy relationship metadata is not used as the visible verdict.

AI, aichat, app and HTTP tests and vet passed. Two focused frontend test files
passed (10 tests); the production frontend build passed. Browser comparison
fixtures/assertions were updated, but Playwright was not executed in this
follow-up. No browser E2E pass is claimed.

Two isolated real-model checks used the reported full-text article pair and
the owner's configured Ofox model. The initial answer used source handles in
its sentence; the prompt was clarified to prefer article descriptions. The
final run returned a single Chinese sentence describing the different topics
in about 6.4 seconds, with no `relation` field, one provider call and 9 verified
citations whose exact ranges all reopened successfully. See
[d-comparison-answer-smoke-20261122.json](d-comparison-answer-smoke-20261122.json).
This verifies this regression scenario only, not NFR-04 accuracy acceptance.

### Source-binding regression fix (2026-11-22)

A live article comparison attributed a film quotation to a HarmonyOS article.
The structured response schema previously allowed every source to choose from
the global union of passage handles. It now binds each source handle to only
its own passage IDs, uses visibly scoped IDs such as `S1-P1`, supplies source
titles as untrusted metadata, and disallows citations for unsupported findings.
Server verification remains authoritative when a provider ignores the schema.
Cross-source references receive a specific UI explanation; the unaffected
source keeps its independently verified result.

Regression checks cover two and three source schema branches, a source with no
passages, and a foreign passage even when both sources contain identical text.
AI, aichat, app and HTTP package tests and vet passed; the two comparison UI
test files passed (8 tests), and the production frontend build passed with the
existing chunk-size warning. An initial uncached Go build ran out of disk
space; removing only its temporary cache and reusing the existing cache passed.

An isolated database-copy smoke test used the owner's two saved full-text RSS
articles, the question about matching topics, and local `gemma3:4b`. It returned
HTTP 504 after approximately 30 seconds (`ai_chat_failed`), before a comparison
result was available. This is not a successful model-accuracy test; no NFR-04
improvement or reliable semantic comparison is claimed. The local model reported
a 4096-token context window. No model settings or owner article data were changed.

### Different topics regression (2026-11-22)

The original four relationship labels could not express different article
topics without treating them as conflicting factual assertions. The prompt
also called compatible findings Same. The model can now return `different`,
which the server preserves and the UI labels Different. Same requires
equivalent answers; merely supported or non-conflicting findings are not Same.
The four existing requirement categories remain available. Quotation matching
is explicitly distinguished from semantic agreement in the result UI.

The added regressions preserve different topics with verified source quotes,
cover Same/Different/Conflict and partial-failure precedence, and check the UI
label. AI, aichat, app and HTTP tests and vet passed; the two focused UI test
files passed (9 tests) and the production frontend build passed with its
existing chunk-size warning.

A real-model smoke used the same two saved full-text articles shown in the
reported failure, an isolated database backup, the question about whether their
topics agree, and the owner's configured `openai/gpt-5.4-mini` through the
Ofox endpoint. It returned `different` in approximately 5.1 seconds. The source
findings described HarmonyOS application development and film/TV recommendations;
all 10 citations matched their original sources and all 10 evidence reopens
returned the exact quoted ranges. One provider call was started. The compact
record is [d-comparison-topic-smoke-20261122.json](d-comparison-topic-smoke-20261122.json).
This is one manually checked regression, not NFR-04 acceptance or an accuracy
score. The pre-fix observation was the user screenshot, not a controlled replay.

### Earlier implementation checks

- Focused comparison Go tests passed.
- Full Go regression passed with the already populated temporary module cache.
  The first default-cache attempt could not extract the existing desktop
  `purego` dependency due filesystem sandbox permissions; it was not a product
  failure. The full rerun included the desktop command and layering checks.
- `go vet` passed for AI, aichat, retrieval, app and HTTP packages.
- Race checks passed for those packages.
- Frontend Vitest: **70 files, 296 tests passed**.
- Production frontend build passed, with the existing bundle-size warning.
- Requirements and AI-use logs remain separate from acceptance approval.
  D's historical and follow-up use is disclosed in the shared AI-use log;
  the raw conversation export is still pending.
- YAML parsing and the new D document links passed. D's two AI-use entry IDs
  are unique. The baseline already contains two `B-AI-014` entries; this
  existing shared-log ambiguity is recorded without renumbering another
  member's historical entries. Requirements/schema files were not changed.

The offline runner can reproduce the focused functional checks. Full suites
and production build commands remain in the implementation plan.

## Browser checks and limitation

The two comparison Playwright journeys were attempted, but Chromium could not
start: macOS bootstrap/Mach-port permission was denied by the sandbox before
page assertions. They are **not counted as passing E2E tests**.

The in-app browser exercised an isolated real application/HTTP/SQLite server
with synthetic saved articles, one captionless video and one image note, and
an explicitly scripted provider. Observed outcomes:

1. The image note remained visible, disabled, with an unsupported-type reason.
2. The captionless video showed preparation and transcription-setting links.
3. One readable article plus a captionless video returned one verified fact
   and one explicit missing-text result, with insufficient evidence overall.
4. Saving an original caption and refreshing readiness enabled comparison of
   both sources; both quotations could be verified.
5. Opening the video quotation highlighted its original text and showed 12.0 s.
6. After editing the article, reopening its old quotation changed that source
   to insufficient evidence and removed its verified jump. The other source's
   verified jump remained available.

An owner-local JPEG records the comparison and original-caption highlight.
It contains synthetic data and a visibly labeled fixture model. This manual
check does not replace the blocked Playwright run or measure model quality.

## Live-model scope and remaining acceptance

Local Ollama `gemma3:4b` was smoke-tested with two synthetic refund policies.
The initial follow-up request returned insufficient evidence for both sources.
After preserving fact-first schema property order, one source was supported
with a verified quotation and the other remained insufficient. Neither smoke
output is a correct full conflict classification for that case. The earlier
unit-equivalence failures remain recorded in the plan. No failed sample is
removed from or reclassified as a passing NFR measurement.

NFR-04 still needs the frozen baseline and two complete 20-case integrated
runs, pinned model/prompt/configuration, independent scoring and review. The
functional tests use controlled responses for workflow/invariant coverage;
their pass counts must not be reported as semantic accuracy.

Independent implementation review and any missing historical test-approval
evidence remain pending. This document records observed checks rather than
retroactively asserting approval.

## Focused-answer stability follow-up, 2026-11-22

The owner's repeated film-question screenshots exposed a coupling between
selected-source evidence states and the top answer: an unused insufficient
row withheld an otherwise supported focused answer. Runtime bundle inspection
confirmed the owner server already contained the previous relevance change.
Two baseline real-model reruns succeeded, so those are not counted as live
reproductions of the screenshot failure. Controlled tests reproduce the exact
state combination and vary unused supported/unrelated/insufficient/read-failed
rows while checking the same focused answer and one provider call.

The model now declares `answerSources`; only their verified findings gate the
answer. Unknown, duplicate and empty dependencies are rejected; missing legacy
metadata retains the conservative all-source check. Used stale/wrong/cross-source
evidence still withholds the sentence. Unused failures stay visible, and a
failed reopen clears the sentence only when its source is a dependency.

During intermediate live checks, two format failures occurred. A diagnostic
proxy captured one response with source IDs `[S2, S2, S1]`, omitting S3. The
old array schema allowed this membership error. The final schema uses one
required object member for each selected handle, each bound to its own ID and
passages. Decoding rejects duplicate keys, missing identities and mismatched
row IDs; legacy arrays retain strict membership validation. Instructions were
condensed so the existing 8,000-character long-source retrieval regression
continues to find middle-of-document evidence.

Final validation: AI, aichat, app and HTTP tests and vet passed; the browser
fixture server compiles; 12 focused UI tests and production frontend build
passed. Three consecutive requests for “影视那篇推荐了哪些类型的作品？” and one
all-source topic question returned answers, using one provider call each.
All 31 final citation reopens matched exact source ranges. Film answers varied
between medium/region categories and genres; these are not scored as exhaustive
semantic answers. See `d-comparison-scope-smoke-20261122.json` for observations,
the intermediate duplicate-source response and usage coverage.

These four final smoke requests do not establish NFR-04 accuracy or guarantee
deterministic wording. Scope and semantic support are model judgments; the GTA
item still supplies only a saved RSS excerpt. Browser E2E was not executed.

## Server-extracted evidence text follow-up, 2026-11-22

The first baseline rerun of “GTA 那篇主要讨论什么？” reproduced the owner's
failure: the model emitted `《GTA 6》游玩体验` from the title as a quote for
S3-P1. That string does not occur in the saved body. C rejected it while two
other body quotations verified, and the dependent answer was correctly
withheld. Source binding and answer dependencies did not prevent model
transcription of unsupported quote text.

The current model contract requests only per-source `passageIds`, never
quotation strings. The server extracts the complete supplied segment for each
issued handle, then re-reads and verifies its saved source/range/version using
the existing B/C checks. Foreign and unknown handles receive no source text;
there is no substring search across other sources or fallback to the title.
Legacy quote output remains strictly validated, with no silent repair.
Mixed legacy/new formats and repeated or excessive IDs are rejected.

Controlled regressions cover the screenshot's invalid legacy title quote,
exact punctuation/Unicode extraction, repeated substrings, wrong-source IDs
even with identical text, unknown/native/source IDs, same-version edits,
version changes and unreadable ranges after generation. AI, aichat, app and
HTTP tests and vet passed; the browser fixture server compiles; 12 focused UI
tests passed. Integration/browser providers now exercise ID-only selections.

The final configured-model smoke test returned answers for 10 consecutive
GTA requests and two controls (film categories and all-source topics), with
one provider call each and 43 exact evidence reopens. Every captured final
model row contained passage IDs rather than model-written quotes. The
baseline failure and final observations are retained in
`d-comparison-passage-selection-smoke-20261122.json`.

This removes model quote-copy errors without proving semantic entailment.
Some model answers still repeat a high-score claim visible in the title
rather than independently establishing it from body evidence. Returned
answers and verified source ranges must not be reported as semantic accuracy
passes. GTA still has only the saved RSS excerpt. Browser E2E and NFR-04
accuracy acceptance remain pending.

## Owner-requested ten-question repeat batch, 2026-11-22

The original ten short questions were recovered from the chat history and
submitted in their original order, ten consecutive times each, with the same
three saved items and configured `openai/gpt-5.4-mini` gateway model. An
isolated SQLite/settings snapshot and frozen executable preserved conditions;
source versions and executable/source hashes remained unchanged throughout.
No failed result was retried or replaced. The batch made exactly 100 provider
calls, with 1,508,571 total reported tokens.

All 100 API requests completed with HTTP 200. Ninety-seven returned a top-level
answer; three withheld it. All 901 exact evidence reopens succeeded. The
assistant then reviewed answers and per-source findings against the supplied
material: 80 had no observed substantive issue, 14 had wording/coverage/support
warnings, and 6 had definite delivery or unsupported-comparison problems.
These are assistant review judgments, not independently measured accuracy.

The substantive failures were Q7 repetitions 2, 3 and 7: shared properties
were inferred from one source only, or a multi-work film list was described
as discussing a single work. Q9 repetitions 3 and 10 lost partial audience
answers with a limitation because the GTA row was insufficient. Q10 repetition
1 lost a correct inability-to-determine-price answer because S1/S2 were both
marked unrelated and declared as answer dependencies. Warnings include mixing
elderly-care cases, repeating title-only superlatives, uncertain source/title
attribution, and overconservative audience inference.

Compact results are in `d-comparison-100-repeat-20261122.json`. The owner-local
artifact directory linked there contains all 100 API results, all 100 model
responses, 20 annotated warning/failure cases, the original question manifest,
review criteria, per-question counts and a reproducible HTTP runner. The mean
request time including evidence reopens was 5.83 seconds. Product code was not
changed during this diagnostic batch. GTA still supplies only the saved RSS
excerpt; browser E2E and independent NFR-04 scoring remain pending.

## Owner-requested unconditional answer display, 2026-11-22

The owner explicitly requested cancellation of the rule that hid a generated
sentence unless its answer-source dependencies passed verification. The
backend now preserves the decoded model answer independently of all per-source
states; dependencies are display metadata only. A failed citation reopen now
updates that source/version and its citation status without clearing the
sentence. Source checks, failed-source reasons and invalid-jump controls remain.
The empty-answer placeholder now describes missing model output rather than
an inability to produce a fully verified answer.

The captured 100 model responses from the prior batch were replayed, in their
original order, through the updated production HTTP/SQLite stack and a local
fixture provider. Each supplied-evidence hash matched the original manifest.
All 100 model sentences were retained exactly, including the previously
withheld sequences 83, 90 and 91. Every per-source verification result matched
the original batch. This was deterministic local replay, with zero additional
paid provider calls, not 100 new real-model generations.

Backend tests and vet passed for AI, aichat, app and HTTP. Thirteen focused
frontend tests and the production frontend/backend builds passed. Coverage
includes all-unverified/all-unrelated source states, invalid or foreign quotes,
unreadable/stale source ranges, preserved error notices, and answer retention
after failed reopen. The Playwright assertion was updated but browser E2E was
not run. Replay observations are recorded in
`d-comparison-answer-display-replay-20261122.json`.

This is the requested display-policy change. It does not correct model
reasoning or resolve the semantic warnings/errors in the original batch.

## Twenty-question live repeat batch, 2026-11-23

The owner requested ten additional questions plus the original ten, with ten
consecutive real generations for each. The 200-request batch used the fixed
post-display-policy executable, the same selected saved items and configured
Ofox `openai/gpt-5.4-mini` alias. All calls were sequential, with no replacement
retries or product/config/source changes during the batch. Every supplied
evidence payload had the same hash, and source versions stayed fixed.

All 200 requests completed with HTTP 200, and every generated model sentence
was retained exactly. All 1,457 citation reopens matched saved text. The batch
made exactly 200 provider calls with 3,016,518 reported tokens; mean elapsed
time including reopens was 5.26 seconds. Twenty-three cases contained source
diagnostic states, which were not automatically treated as answer failures.

Assistant source-guided review recorded 172 cases with no observed substantive
issue, 16 wording/coverage warnings and 12 definite answer problems. Nine of
the problems occurred in Q20, where the model claimed that GTA had only the
13-year figure despite its saved editorial note explicitly containing a
10-point score. Another Q20 response incompletely summarized the two values.
Q16 had independently located the 10-point score in all ten generations from
the same input, confirming that the numeric information was available.

Other definite cases were Q7/R6 (a shared plot-summary claim unsupported by
the GTA excerpt), Q17/R8 (calling the third GTA source the second source),
and Q18/R4 (claiming duration disagreement while acknowledging no GTA
duration). Warnings include care-app grouping, unsupported age/platform
specificity, title/body coverage, and incomplete conservative audience answers.

The original twenty questions, all API/model results, 28 annotated cases,
source proof, per-question review and the reusable runner are linked from
`d-comparison-200-repeat-20261123.json`. These are targeted diagnostic results,
not independent human scoring or NFR-04 acceptance. GTA still has only its
saved RSS excerpt; no browser E2E was run. The cancelled answer-suppression
rule was not reintroduced, and product logic was not changed in this batch.


## Video comparison scope and state handling

Comparison prompts carry source kind, saved subtitle coverage bounds and item
video duration. Saved-text completeness is explicitly distinct from full-video
coverage. Focused questions first determine applicable sources; subtitles do not
offer the structured-article absence state. The prompt preserves uncertain ASR
values and distinguishes discovery, authorship and verification. Generated answers
remain visible independently of citation checks; exact quote validity is still
not semantic proof. A deterministic scope/schema regression checks the 455-second
video with only 0.16–90.14 seconds supplied and retains ambiguous ASR unchanged.

The retained real-model NFR evaluation is available in
[the complete evaluation record](d-nfr-full-repeat-20261125-002.json), with
all 120 responses, source snapshots, freezes, score rationales and regression
logs under `build/d-nfr-full-20261125-002`. This is owner-delegated assisted
local evaluation, not independent review.
