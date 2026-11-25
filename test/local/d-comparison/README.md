# D · Local NFR-04 self-evaluation

The [NFR-04 specification](../../../docs/req/nfr-04-local-evaluation.md) permits
author-run, author-scored evaluation on this machine. It needs Python 3 and the
existing Go/Node dependencies for the controlled regression. The author
prepares the cases, runs both rounds, scores findings against the originals and
archives the report using Python standard-library tools.

Evaluate **article/article**, **video/video** and **video/article** as separate
20-question profiles. The quality threshold is **16/20 answer scores and 16/20
case-level attribution scores in each of two full rounds of every profile**. Passing the controlled regression is also
required. Neither quotation matching nor scripted responses establish semantic
accuracy. Results are labeled `author_self_evaluation`.

## 1. Prepare and freeze the expected findings

Run commands from the repository root. Each profile has a separate local evidence
directory, manifest, 20-case set and 40-row score sheet. The commands below show
one profile; execute the same sequence for the other two with their own
directories. Keep each profile and round separate when reporting results.

Use a new local evidence directory:

```sh
python3 test/local/d-comparison/evaluate.py init --dir ./build/d-nfr-04-run-001
```

Fill `manifest.json` with your name, actual saved conversation model and settings
revision, generation/context/retrieval settings, and source IDs. Each source has
`kind` (`article` or `subtitle`), a saved source/track version, and the path to a
UTF-8 copy of its original text. Save those copies under `snapshots/`. Do not
include credentials. These copies must match what the local application reads;
verify the saved original track selection for videos. Record its language,
revision, cue count, time coverage and full item duration. Partial subtitles
support only their supplied text; do not assume they cover an entire video or
unseen parts. Article/article cases contain articles, video/video cases contain
video subtitle sources, and video/article cases contain both.

Fill all 20 rows in `cases.csv` **before collecting responses**:

- `case_id`: D-01 through D-20 in order, once per quality set.
- `question`: the actual question to ask.
- `source_ids`: two or three distinct saved IDs, separated by `|`, in the desired order.
- `expected_answer_points`: source-based required findings, values, conditions
  and acceptable uncertainty. For example, “A allows 14 days after payment;
  B allows 30 days; the deadlines differ.”
- `expected_source_points`: expected facts/evidence/relevance for each selected
  ID, original passage or subtitle time, and legitimate limitations.

Use representative all-source and focused questions. Record actual text-type
coverage. An article-only quality dataset does not establish video-model quality.
Do not choose only previously passing questions or derive expected findings from
the new run's answers. Gold notes are never sent to the model.

```sh
python3 test/local/d-comparison/evaluate.py freeze --dir ./build/d-nfr-04-run-001
```

This validates the templates and records text/configuration/implementation
hashes in `freeze.json`. It starts no model call. Changing frozen material or
production source files requires a new directory and two new complete rounds.
Keep the prior directory, including failures.

## 2. Collect both rounds

Run the application locally using a build of this checkout and the frozen saved
model/settings. The collection command uses an **already running** local API;
it does not start a server. Its default endpoint is `http://127.0.0.1:7380/api`.

```sh
python3 test/local/d-comparison/evaluate.py collect --dir ./build/d-nfr-04-run-001 --run 1
python3 test/local/d-comparison/evaluate.py collect --dir ./build/d-nfr-04-run-001 --run 2
```

Use `--api http://127.0.0.1:PORT/api` if the application uses a different port.
Only loopback application URLs are accepted. The configured model can still be a
remote provider; collection is a real model evaluation and may incur its normal
cost. A local model can be used for a fully local setup.

Each round submits the 20 cases sequentially and saves the exact decoded API
response plus collection metadata as `run-1/D-01.json` through
`run-2/D-20.json`. HTTP failures and request failures are retained. Existing
response files are preserved; repeating the command resumes only missing cases,
without silently replacing failed observations. Round 2 requires all first-round
responses. No expected-answer/scoring fields are included in the requests.
An attempt marker is saved before each request. If collection is interrupted
during a request, that unknown/failed attempt is retained and scores 0 rather
than being silently sent again.

The collector checks the saved model and settings revision before the round.
Do not change sources, track selection, model, configuration or the running
implementation during the rounds. Snapshot hashes verify the retained local
copies, not the application database; the author must also check versions and
the actual original evidence returned by the application.

## 3. Score the answers against the originals

Read each response, its source findings and original evidence, then fill the
40 existing rows in `scores.csv`:

- `answer_correct`: 0 or 1 under the NFR answer rubric.
- `source_attribution_correct`: 0 or 1 for the complete set of selected sources.
- `notes`: the source-based rationale, including any failure or mismatch.

Keep separate scores even when an answer is correct but the evidence attribution
is wrong. Blank answers and request failures score 0 rather than disappearing
from the denominator. A generated answer remains visible under FR-04 even when
the evidence or semantic quality receives a failing score. Do not use the
verified badge as proof of semantic correctness.

## 4. Run controlled faults and generate the report

```sh
python3 test/local/d-comparison/evaluate.py regression --dir ./build/d-nfr-04-run-001
python3 test/local/d-comparison/evaluate.py score --dir ./build/d-nfr-04-run-001
```

`regression` runs the existing [offline comparison runner](../../offline/d-comparison/README.md),
which covers failure isolation, missing/limited text, invalid evidence, stale
sources and answer retention. It starts no real model or platform call and does
not modify saved application data. Go tests use local loopback test servers.
Its actual output, exit code and freeze/log hashes are saved in
`regression.log` and `regression.json`. Existing regression evidence is not
overwritten. A failing run remains evidence; diagnose it and use a new evaluation
directory/configuration for a fresh acceptance attempt.

`score` writes `report.json`, recording each round's two numerators with a fixed
denominator of 20, missing evidence, response hashes and the regression state:

- Exit 0 / `PASS`: this profile's metrics pass in both rounds, all rows have scores and
  rationale, all raw responses are present, and controlled regression passes.
- Exit 1 / `FAIL`: complete evaluation with a failed threshold or regression.
- Exit 2 / `INCOMPLETE`: missing grades, evidence, regression or invalid/frozen
  materials. The result cannot count as an acceptance pass.

The tool checks response format, model, source identity/order and record
completeness. It does **not** automatically grade semantic correctness, validate
that an author actually inspected a quotation, or provide independent review.
Each report covers its own profile. Overall NFR-04 completion requires passing
reports for all three profiles. Reports are local self-evaluations, not PR approval. Archive the whole
directory and note uncovered input types and known limitations in the report.

## Scoring-tool checks

```sh
python3 -m unittest discover -s test/local/d-comparison -p 'test_*.py' -v
```

These tests use synthetic data and mocked collection/regression responses. They
test the evaluator itself and are not a measured NFR-04 quality pass.

## Published evaluation bundles

The [PR12 measurement](../../../docs/evidence/d-nfr-pr12-20261125-001.json)
archives the current implementation, model revision 8, all 120 responses and
all grades, including failures. Verify its frozen implementation bytes, original
text hashes, response hashes and per-round arithmetic without credentials:

```sh
python3 test/local/d-comparison/verify-public-evaluation.py --archive docs/evidence/d-nfr-pr12-20261125-001.zip
```

The earlier retained attempt remains a separate historical bundle, verified by
`python3 test/local/d-comparison/verify-retained.py`. These integrity checks do
not independently validate the assisted semantic grades or waive missing older
attempts. Session evidence and its explicit redactions are documented in
[the transcript directory](../../../docs/evidence/d-session-export/README.md).
