# D · Multi-content comparison: functional contract and evaluation plan

- **Owner:** Wenlong Lu (D).
- **Status:** Functional implementation and local self-evaluation records available; latest complete assisted local repeat meets NFR-04 thresholds; implementation review pending.

The [FR-04 specification](../req/fr-04-multi-content-comparison.md) defines the functional behavior and acceptance criteria. The [requirements registry](../req/requirements.yaml) records FR-04 and NFR-04. The [local self-evaluation protocol](../../test/local/d-comparison/README.md) defines the quality dataset, collection and scoring records.

## 1. Functional requirement FR-04

A reader selects two or three distinct accessible saved articles or videos and asks a question about all or a subset of them. The configured enabled conversation model returns one concise natural-language sentence in the user's language. The answer is one line and at most 600 Unicode characters. Overall output is free text; source cards retain structured evidence states.

Article evidence comes from saved body text. Video evidence comes from a saved original-language subtitle track, whether supplied by the platform or produced by transcription. Article/article, video/video and mixed selections are supported. Images, PDF, standalone audio, video frames, automatic transcription during comparison and persisted multi-turn comparison history are outside FR-04.

### Source selection and preparation

The comparison panel displays the selection count, titles and saved-text readiness. Unsupported content is visible with a disabled choice and reason. Missing text has a preparation link. Source inspection reads saved data only and starts no model, platform fetch, download or transcription.

When a video has multiple original subtitle tracks without a saved selection, the panel provides track information and an inline selection/save action. Translated and mismatch-flagged tracks are excluded. Preview or limited coverage is disclosed. Saving checks the current revision; a concurrent change requires refresh rather than overwriting a newer selection. Changing a saved track clears the previous comparison result. Users can refresh readiness after preparing text. No-ready selections cannot be submitted in the UI; a selection with at least one ready source can produce a partial result.

Submission independently validates the question, two/three-source count, unique source IDs, access, model configuration, revision and optional call limit before generation. Missing and unauthorized IDs share a generic public error. The server reloads the saved selections, source snapshots and versions; readiness is advisory.

### Answer and evidence display

Every selected source appears exactly once in selection order. The model receives invocation-scoped source and passage handles. Facts address the requested question dimension, and answer references retain the same source identity. Source references are rendered from the immutable ID-to-title mapping. Title questions, conditions, values and uncertainty must remain intact; a question about inconsistencies does not imply that one exists.

Source evidence states are `supported`, `not_mentioned`, `not_relevant` and `insufficient_evidence`. A supported finding carries its own verified original evidence. A confirmed absence requires complete inspection of explicitly complete structured fields with a versioned coverage record. No search hit or partial excerpts do not prove absence. An unrelated source requires complete supplied, revalidated text and carries no fact or citation. Missing text, failed reads, incomplete coverage, invalid quotations and changed versions retain explicit reasons rather than being hidden as unrelated.

Once model output is successfully decoded, its answer is displayed regardless of whether any or all source checks pass. Invalid citations cannot receive verified status or verified navigation, but do not clear the answer. Reopening stale or deleted evidence updates only the affected source's evidence state and navigation; other findings and the generated answer remain visible. When no source has usable text, zero generation requests start and source issues are shown. Provider failures and malformed output have explicit task errors; the service does not invent an answer.

A verified citation opens the article passage or subtitle excerpt, with the original locator and subtitle time where applicable. Reopening rechecks the saved version and exact text. Source/version/quotation matching does not establish semantic support or model accuracy. Comparison does not modify single-content chat, history or clear behavior.

### Example results

- **Three-source question:** “Do all three sources give the same refund deadline?”
  **Answer:** “Store Help and Customer Guide allow 14 days after payment, while Updated Terms allows 30 days.” Each card shows its own finding and original evidence.
- **Focused question with three selected items:** “What does the game-review article discuss?”
  **Answer:** A sentence summarizing that article, with other source cards showing their own relevance or readiness status.
- **Partial evidence failure:** A generated sentence remains visible alongside the failed source's reason and the remaining valid citations. It is not replaced by a fixed overall relationship label.

## 2. Integration contracts

- `POST /api/ai/compare/sources` accepts 1–100 distinct saved IDs and returns readiness in input order without requiring model configuration. It reports ready text, required track selection, missing text, unsupported kinds, oversized units and generic unavailability.
- `POST /api/ai/compare` accepts `contentIds`, `question`, AI settings `revision`, optional enabled `model`, optional `maxProviderCalls` and optional `maxChars`. A call limit is omitted for unlimited calls or supplied as an integer from 1 through 100. `maxChars` is an integer from 1 through 120000. The response carries the answer, execution status, source identities, ordered per-source findings, citation/coverage information and invocation accounting. Compatibility relationship metadata is not the user-facing answer contract.
- `POST /api/ai/compare/evidence` accepts a returned verified citation and returns a freshly checked original passage, highlight offsets and locator. Subtitle evidence includes time information where available. Invalid, stale, deleted or forged references cannot enable verified navigation.
- B supplies source passages, locators, versions and bounded retrieval. Short complete sources are supplied directly; longer sources use question-relevant retrieval. Instructions, schema, question and source context count toward character and UTF-8 byte limits. Context is allocated across selected sources, and inadequate coverage is disclosed.
- C verifies original evidence after the source is re-read. Source handles and passage IDs are bound to the selected snapshots; unknown, duplicate and cross-source identities are invalid. The server extracts exact text from issued passage handles and validates its source, version and locator.
- A supplies one invocation ledger shared by generation, retries and format fallbacks. The usual structured comparison uses one provider completion. Every additional completion requires a reservation on the same ledger. Budget exhaustion, unknown usage, cancellation and provider failures keep their task states. B retrieval calls are accounted separately.

## 3. Nonfunctional requirement NFR-04

The [NFR-04 specification](../req/nfr-04-local-evaluation.md) defines local comparison quality evaluation. The author freezes 20 pre-annotated questions for each article/article, video/video and video/article profile, runs two complete rounds per profile through the local application and configured real model, inspects original evidence, records both per-case scores and produces the self-evaluation report. Course-required test approval, PR review and CI records are maintained separately.

### Frozen quality dataset

Prepare separate 20-question sets for article/article, video/video and video/article inputs, with two or three selected source IDs per question. Include questions about all selected sources and focused questions about a subset, source identity/order, and supported differences or limitations. Record actual text-type coverage, selected subtitle language/revision, cue counts, time intervals and video durations. A partial subtitle track does not establish full-video or other-part coverage. Before collecting output, annotate expected answer points and each source's expected fact/evidence/relevance/limitation from the originals. Do not build the acceptance set by selecting only successful outputs.

Freeze source copies and versions, questions/annotations, the model and saved settings revision, generation/retrieval/context settings, and implementation hashes. Only source IDs, questions and invocation settings go to the application, never expected findings. A changed dataset, source, implementation or configuration requires a new evidence directory and two new complete rounds. Retain the complete evaluation used for final acceptance; keeping every earlier debugging run or superseded evaluation attempt is not required.

### Two real-model rounds and author scoring

Run each 20-case profile through the local application, then repeat that full profile using the same configuration. Keep all three profiles and six round score sets separate. Use the configured real conversation model, which can be local or remote. Within the retained acceptance evaluation, save all raw responses, including HTTP/model failures. The collector uses an already-running loopback application endpoint; it starts no application service and preserves existing observations when resuming an interrupted round.

1. **Answer correctness** = semantically correct answers / 20. Each case scores 0 or 1. The sentence must answer the question with the annotated findings, source identities, values, conditions and uncertainty, without material unsupported claims or omissions. Equivalent wording and unit conversions are allowed; no fixed relationship label is required. Missing/invalid answers and request failures score 0.
2. **Case-level source attribution accuracy** = cases where every selected source is correct / 20. Each case scores 0 or 1. Every selected source must appear once in selection order with the correct identity and finding, grounded citation/locator/version or justified complete-field absence, irrelevance or limitation. One swapped, fabricated or omitted source fails this metric for the case. Quotation matching alone does not prove semantic support.
3. **Author assessment** records separate 0/1 scores and source-based rationale for every case in both rounds. Missing grades or raw evidence make the evaluation incomplete, not passing. The author checks original evidence and citation locations; the statistics script does not judge semantics.
4. **Per-run threshold:** each round must independently score at least 16/20 for both metrics. Do not pool rounds, drop failed cases, or report only the better round. Record known model/prompt settings, call counts, available usage, latency and remaining limitations where available. Results are labeled `author_self_evaluation`.

### Controlled failure gate

Run the existing local offline comparison suite and preserve its output and actual exit code. It must exit zero and cover read failures, missing/limited text, invalid or cross-source citations, stale/deleted sources, and decoded-answer retention. Every injected failure remains explicit on the correct source, zero invalid citations become verified, and valid remaining evidence stays usable. Source failures cannot become unsupported full-source agreement. Scripted provider output is permitted for this deterministic gate only; it does not supply the real-model quality scores.

### Local evidence and report

The [local runner](../../test/local/d-comparison/README.md) supplies `init`, `freeze`, `collect`, `regression` and `score` commands. Its retained files are `manifest.json`, `cases.csv`, original snapshot copies, `freeze.json`, two directories of raw responses, `scores.csv`, `regression.log`, `regression.json` and `report.json`. It verifies frozen file hashes, response membership/model/format, case completeness, separate round totals and the regression record. Semantic grading remains the author's responsibility.

Report **NFR-04 local self-evaluation passed** only when both real-model rounds in all three profiles meet both thresholds and each recorded controlled regression passes with complete evidence. The retained evaluation must include all 120 scheduled observations, including failures, together with frozen inputs/configuration, score rationales and regression records. Record its actual failed or incomplete outcome if any gate is unmet; do not remove failed cases or assemble rounds from different attempts. Earlier debugging runs and superseded evaluation attempts do not have to be archived. No measured quality pass is asserted by this plan.

## 4. Acceptance traceability

- **D-T01 / AC-FR-04-1:** Two/three-source selection, saved-text inspection, inline original-track selection, revision conflicts, readiness refresh, disabled unsupported choices and validation before provider calls. Inspecting saved sources must not fetch media or invoke AI.
- **D-T02 / AC-FR-04-2:** One-sentence natural-language answers for all-source and focused questions, ordered unique source findings, stable attribution, article/subtitle/mixed input, verified original evidence and complete-field absence. Source evidence states are distinct from the overall answer.
- **D-T03 / AC-FR-04-3:** Read failures, missing text, incomplete retrieval and changed versions expose the correct source reason. Other findings remain. Decoded answers stay visible even when every source check fails. All-unusable sources start zero generation calls.
- **D-T04 / AC-FR-04-4:** Reject cross-source, altered, forged, stale and out-of-range evidence from verified status/navigation. Duplicate or unknown source identities and malformed output produce explicit errors. Invalid citation checks alone do not remove a decoded answer.
- **D-T05 / AC-FR-04-5:** Shared A ledger across compatibility fallbacks, valid/invalid/omitted call limits, usage-unknown/cancellation states, separate B counts and complete character/byte accounting including the schema. Limited coverage is explicit.
- **D-T06 / NFR-04:** Three frozen 20-question profiles, two complete real-model rounds per profile, author semantic/source scoring with both 16/20 thresholds in every profile and round, and a passing recorded controlled failure regression. Evidence is labeled author self-evaluation.
- **D-T07 / AC-FR-04-6:** HTTP/SQLite composition, correct article and subtitle evidence locators, refreshed stale/deleted evidence without clearing the answer, invalidation after track selection changes, unchanged single-content chat/history and browser interaction.

## 5. Verification and acceptance records

The [functional verification record](../evidence/d-fr-verification.md) contains executed checks and diagnostic limitations. The [offline runner](../../test/offline/d-comparison/README.md) reuses canonical Go and frontend tests. Deterministic provider fixtures establish functional behavior; diagnostic repetitions and citation reopens do not replace the frozen local quality set, two complete real-model rounds and explicit author semantic scoring.

NFR-04 acceptance requires Section 3 to pass with all evidence for the retained evaluation available. The [current complete assisted evaluation](../evidence/d-nfr-pr12-20261125-001.json) meets both 16/20 thresholds in both rounds of all three profiles, with all controlled regressions passing. Scoring was prepared by Codex under the owner's standing original-text grading delegation; no fresh independent human scoring or PR approval is claimed. Residual semantic and presentation findings remain disclosed. Course/project-required test-plan approval, PR review and CI records remain separate. Browser checks that did not execute must not be reported as passing.

The quality dataset, source copies, configuration and implementation are identified by the hashes in `freeze.json`.
