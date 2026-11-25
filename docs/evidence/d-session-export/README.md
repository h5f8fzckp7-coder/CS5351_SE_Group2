# Codex conversation evidence

This is the sanitized chronological projection of the owner's actual Codex
session, through the cutoff recorded in `manifest.json`. It includes user and
assistant messages and original tool calls/results. Every original record has
its ordinal and the SHA-256 of its raw JSONL line; removed private content has an
explicit typed tombstone. The complete private raw export is retained locally,
with byte count and hashes in `../d-codex-session-provenance.json`.

`conversation.sanitized.jsonl.gz` is UTF-8 JSONL compressed with gzip. Read it
with a gzip-capable text viewer. `student-interventions.csv` indexes every actual
user message by original record number and timestamp, with a sanitized excerpt.
The annotations were prepared by Codex to locate actual interventions; they do
not turn a user request into an independent requirement approval, test-plan
signature, code review or instructor waiver. Owner acceptance of this annotation
presentation remains pending.

The manifest explicitly describes media, credential, home/temporary-path, runtime-policy,
encrypted-reasoning and duplicate-event redactions. This is not a byte-identical
public copy of the raw session and cannot recreate deleted historical test
response files. Media attachments are identified by hashes rather than republished.

The recovered runtime model label is `gpt-6.1-sol`; immutable provider build ID is
unavailable. This is Codex evidence, not OpenCode evidence. Course acceptance of
the tool used requires an actual course decision; none is claimed here.

From the repository root, run:

```sh
python3 test/local/d-comparison/verify-session-export.py
```

This verifies chronology, the public archive checksums, the complete user-message
index and common credential/home-path patterns. It does not prove independent
semantic review or replace a human review of the sanitized export.
