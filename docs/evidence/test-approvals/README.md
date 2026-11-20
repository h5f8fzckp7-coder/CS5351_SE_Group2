# Test-approval records

The test-approval CI gate reads YAML files in this directory. A record is valid only when a **non-author** reviewer has set `decision: APPROVED` and named the commit that contained the approved plan and tests.

Do not self-approve. The Owner E template below is pending and is not an approval.

Copy `owner-e.pending.yaml` to a dated file, fill the reviewer fields, and point `approved_commit` at the documentation/test commit that was reviewed, which must be an ancestor of the implementation commits.
