# NFR-04 · Local comparison quality evaluation

- **Owner:** Wenlong Lu (D).
- **Type:** Nonfunctional requirement.
- **Priority:** Must.
- **Verification:** Author-run local evaluation with original-text inspection and author scoring.
- **Verification status:** Latest complete owner-delegated AI-assisted local repeat meets all six per-round quality gates and controlled regressions. See the [latest full record](../evidence/d-nfr-pr12-20261125-001.json). Residual semantic/format defects and independent review pending status remain disclosed.
- **Functional scope:** [FR-04](fr-04-multi-content-comparison.md).
- **Execution guide:** [Local evaluation instructions](../../test/local/d-comparison/README.md).

## Requirement statement

Evaluate three input profiles: article/article, video/video and video/article. Each profile has its own fixed set of 20 pre-annotated questions and two complete rounds through the local application using the same configured real conversation model. Each metric in every profile and round must reach at least 80%: at least 16 correct cases out of 20 for answer correctness and case-level source attribution accuracy. Score profiles and rounds separately; do not pool results or report only the better round.

The local controlled regression must correctly disclose source read failures, missing text, limited coverage, invalid citations and source version changes. No invalid citation may receive verified status or verified navigation. Source problems must not be concealed as a complete agreement conclusion. Successfully decoded answers remain displayed under FR-04.

## Test environment and materials

The author prepares the dataset, runs the application locally, checks the original text, scores the outputs and produces the report. Label results **author self-evaluation**, recording the actual evaluator, model configuration and tested scope.

Quality runs call the configured real conversation model through the locally running application. The model may run locally or use a remote service. A remote service can require network access and incur charges. The evaluation script uses an already-running application; it does not start or modify the service or read or save model credentials.

Before collection, freeze 20 questions for each profile, two or three accessible saved source IDs per question, original text copies and versions, expected answer points and expected findings for every selected source. Article/article cases select article sources, video/video cases select video subtitle sources, and video/article cases include both types. Keep a separate evidence directory and score sheet for each profile. Cover all-source and focused questions, source identity and order, and evidence-based differences or limitations. Derive expectations from the original text, not generated answers, and do not select only cases known to pass.

Saved article text and original-language video subtitle text are eligible. Record the selected language, track revision, cue count, subtitle coverage interval and video duration. A saved preview or partial track supports claims about its supplied text, not the entire video, unseen parts or the soundtrack. Prepare and select tracks before collection; transcription accuracy and video-frame understanding are outside this comparison quality metric. The report identifies the actual sources, content types and model evaluated, and quality claims are limited to that scope.

Frozen materials identify the questions and annotations, original-text hashes, source versions, model, settings revision, invocation/context/retrieval configuration and implementation hashes. A change to the dataset, scoring criteria, implementation, source or configuration requires a new evaluation directory and two complete rounds. Retain the complete evaluation used for final acceptance. Retaining every debugging run or superseded evaluation attempt is not required.

## Scoring method

### Answer correctness

Score each case 0 or 1. Award 1 when the answer directly addresses the question, includes the pre-annotated essential findings, preserves source identity, values, conditions and uncertainty, and contains no material unsupported claim, omission or contradiction.

Allow semantically equivalent natural language and unit conversions. No fixed overall relationship category is required. A focused question needs an answer about its named source or subset. An all-source question cannot receive full credit for a complete conclusion based on only one source.

Wrong, blank or malformed answers and model/request failures score 0. Exact quotation matching does not establish semantic correctness.

### Case-level source attribution accuracy

Score each case 0 or 1. Award 1 only when every selected source has the correct identity, order, relevance or limitation state, and its findings, citations, locators and versions belong to that source. One swapped, fabricated, omitted or incorrectly attributed source gives the case 0 for this metric.

Check whether each `supported` finding is supported by its own original text, whether `not_mentioned` has complete-field coverage, whether `not_relevant` matches the question scope, and whether `insufficient_evidence` states a real limitation. Open and inspect the original evidence; a verified badge alone is not a semantic score.

### Denominators and records

The required archive covers the final acceptance evaluation: all three profiles, both rounds, all 120 scheduled responses (including errors and incorrect answers), frozen materials and configuration, complete score sheets with rationale, and controlled-regression evidence. Earlier debugging runs and superseded evaluation attempts may be omitted from this archive. Do not combine selected cases or rounds from different attempts into the retained evaluation.

Each metric has a fixed denominator of 20 for each profile and round. Do not exclude failed cases. Record both scores and a source-based rationale for every case. Missing cases, grades, raw responses or controlled-regression evidence make the evaluation **incomplete**, not passing. Errors in a complete record score 0. Any metric below 16/20 in any profile or round makes the evaluation **fail**.

## Acceptance criteria

### AC-NFR-04-1 · Profile-specific local quality rounds (D-T06)

The author runs and scores each of the three frozen profile sets twice, with 20 cases per profile per round: 60 distinct profile cases and 120 responses in total. Every profile and round independently reaches at least 16/20 for answer correctness and at least 16/20 for case-level source attribution accuracy. Archive all raw responses, model and implementation configuration, material hashes, all six complete score sets and per-case rationale. Label the report author self-evaluation.

### AC-NFR-04-2 · Failure disclosure and evidence validity (D-T06)

The local controlled comparison regression exits with code 0 and covers failed reads, missing original text, limited coverage, invalid citations, and stale or deleted sources. Every injected failure appears on the correct source. Zero invalid citations receive verified status or verified navigation. Other valid evidence and generated decoded answers remain available under FR-04.

Quality runs use the real configured model. Scripted provider responses are used only for controlled regression, not as measured real-model quality scores.

## Completion

Record **NFR-04 local self-evaluation passed** only when both quality metrics pass in both rounds of all three profiles, each profile has passing controlled-regression evidence and the complete final acceptance evaluation is retained. Report unexecuted checks, uncovered scenarios and known limitations separately. Maintain course-required test-plan approval, change review and CI records as their respective project records.
