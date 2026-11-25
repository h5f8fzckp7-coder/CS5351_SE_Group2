# FR-04 · Multi-content comparison

- **Owner:** Wenlong Lu (D).
- **Type:** Functional requirement.
- **Priority:** Must.
- **Acceptance status:** Independent implementation review and acceptance pending.
- **Registry:** [requirements.yaml](requirements.yaml).
- **Test and integration plan:** [D comparison plan](../plans/d-comparison-plan.md).

## Requirement statement

A reader can select two or three distinct, accessible saved articles or videos, ask a question about the selection, and receive one concise natural-language sentence from the configured conversation model. Each selected source has a result showing its finding, original evidence, citation verification and source limitations.

The question may concern all selected sources or only one source or a subset. The answer directly addresses the question and preserves source identity, values, conditions and uncertainty. Overall output is natural language rather than a fixed relationship category.

## Content scope

1. Articles use saved body text. A card title or description cannot substitute for original body evidence.
2. Videos use saved original-language subtitle text, including platform subtitles or saved transcription output. Comparison operates on that text.
3. Article/article, video/video and mixed article/video selections are supported.
4. Images, image-note content, PDF, standalone audio and video frames are outside this requirement.
5. Each submission is a single-turn question. Comparison does not write, merge or clear single-content chat history.

## User workflow

The reader opens the comparison panel, searches for saved items and selects two or three. The panel displays the selection count, titles and text-readiness status. Unsupported items show a reason and cannot be selected.

Sources without saved body text or original subtitles have preparation links. When a video has multiple original tracks and no saved selection, the panel displays track information and allows the reader to select and save one for comparison. Translated and mismatch-flagged tracks are excluded. Preview-only or limited coverage has an explicit notice. Saving checks the current revision; a concurrent change prompts refresh without overwriting a newer selection.

Readiness inspection reads saved data only. It starts no model request, media download or transcription. The reader can refresh source status after preparing text or changing a track. The UI disables submission when no selected source is ready. With at least one usable source, submission can produce a partial result.

The reader enters a question, selects a configured enabled conversation model and may set an invocation call limit. Submission revalidates input, access, source status and versions. The result displays the answer first, followed by one source card per selected item in selection order.

## Answer and source results

### Natural-language answer

The model generates one concise sentence from the supplied text, in the question's language, on a single line and within 600 Unicode characters. A focused question is answered from the named source or subset. An all-source question addresses the selected set and its available information and limitations. Source references retain the selected identities; filtering relevant sources must not renumber or swap attribution.

A successfully decoded generated answer is always displayed. Failed citation checks, insufficient evidence, unrelated sources or stale versions do not clear it, even when every source check fails. Reopening unavailable evidence updates the affected source's evidence state and navigation while preserving the answer.

When no usable text exists, generation fails or output cannot be decoded, the system shows the specific source or task status. It does not fabricate an answer when none was generated.

### Per-source results

Every selected source appears exactly once in selection order with its real identity. Evidence states are:

- `supported`: a relevant finding with verified original evidence belonging to that source.
- `not_mentioned`: a field is absent after complete inspection of structured material explicitly declaring a complete field scope; the result includes the source version, inspected scope and field information.
- `not_relevant`: complete supplied and revalidated text is unrelated to the question; the source carries no factual claim or citation.
- `insufficient_evidence`: missing text, read failure, limited coverage, invalid citations or changed source versions, with a specific reason.

Complete-field scope recognition is deliberately limited to these explicit standalone headers (case-insensitive in English, ignoring a trailing colon or full stop): `complete fields`, `all fields`, `the following fields are the complete service card`, `完整字段`, `完整字段如下`, `全部字段`, `以下为此卡完整字段`, `以下是完整规格字段`, `本登记卡所列字段完整，仅包含下列字段`, `本活动登记卡的完整字段如下`, `本卡完整列出所有规格字段`, and `下列字段是本记录全部字段`. Other formulations fall back to limited search, even if a reader would consider them equivalent; they cannot establish absence.

The absence of search results or access to only partial text does not establish field absence. A real source failure cannot be hidden as irrelevance. Citation verification establishes source identity, version, location and exact quotation matching; semantic support for a finding or answer is evaluated separately.

### Original-text navigation

Verified citations open the corresponding article passage or subtitle excerpt with its locator. Subtitle evidence provides the relevant time location. Opening evidence rechecks the source and version. Cross-source, forged, altered, stale or out-of-range references cannot receive verified status or enabled verified navigation. Other valid source citations remain available.

## Acceptance criteria

### AC-FR-04-1 · Selection, preparation and input validation (D-T01)

- Accept two or three distinct, accessible saved article or video IDs.
- Display readiness, original-track selection/save and source refresh; disable unsupported selections and submission when no selected source is ready.
- Before generation, reject blank or overlong questions, invalid source counts, duplicate or malformed IDs, inaccessible sources, invalid model configuration/revision and invalid call limits.
- Missing and unauthorized IDs share a generic public error without exposing access information.
- Readiness inspection starts no model, media-download or transcription request.

### AC-FR-04-2 · One-sentence answer and source attribution (D-T02)

- Accept article text, original video subtitle text and mixed selections.
- Use the configured enabled conversation model to generate one natural-language sentence directly answering the question rather than a fixed overall category.
- Display each source exactly once in selection order; findings, citations and answer references retain correct attribution.
- Support all-source and focused questions. Confirmed absence requires complete-field coverage evidence.

### AC-FR-04-3 · Partial failures and answer retention (D-T03)

- Show the affected source's reason for failed reads, missing text, limited coverage or changed versions; preserve other available findings and evidence.
- Display a successfully decoded generated answer even when any or all source checks fail.
- When no source has usable text, start zero generation requests and report source issues.
- Missing text or failed reads cannot become confirmed field absence or source irrelevance.

### AC-FR-04-4 · Citation and output validation (D-T04)

- References failing identity, version, text or locator checks cannot receive verified status or enabled verified navigation.
- Citation failure alone does not clear a successfully decoded answer.
- Require complete, unique and correct source identities. Unknown, duplicate or mismatched identities and undecodable output produce explicit failures.
- Verified source/text matching does not establish semantic correctness of the model's judgment.

### AC-FR-04-5 · Invocation and context limits (D-T05)

- An omitted call limit means unlimited calls; explicit limits are integers from 1 through 100.
- Generation, retries and compatibility fallbacks share A's invocation ledger and respect budget-exhausted, usage-unknown, cancelled and failed states.
- B retrieval calls are accounted separately. Context satisfies both character and UTF-8 byte limits.
- Identify sources without usable context. Undisclosed truncation cannot stand for complete coverage.

### AC-FR-04-6 · Evidence navigation and single-content chat compatibility (D-T07)

- Verified references open the correct article passage or subtitle time location and recheck current source validity.
- Updated or deleted sources lose affected verified status/navigation; other valid evidence and the generated answer remain visible.
- Changing the comparison subtitle selection clears the previous comparison result; the next submission reads the saved selection and version.
- Single-content chat, history loading and clear behavior remain compatible. Comparison does not modify chat history.

## Acceptance boundary

These criteria define FR-04 behavior and functional acceptance scenarios. The frozen dataset, quality thresholds, two local rounds, author scoring and controlled failure regression are defined by the [NFR-04 specification](nfr-04-local-evaluation.md), [registry](requirements.yaml) and [evaluation plan](../plans/d-comparison-plan.md#3-nonfunctional-requirement-nfr-04). Functional checks, real-model diagnostics and measured quality acceptance have separate records. This specification does not declare an acceptance pass.
