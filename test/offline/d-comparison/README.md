# D comparison functional checks

Run from the repository root:

```sh
sh test/offline/d-comparison/run.sh
```

This entry point runs the real comparison service, retrieval/citation checks,
provider-format compatibility tests, HTTP/SQLite integration, and frontend
presentation tests with deterministic fixtures. Provider responses are scripted;
there are no live model or platform calls. HTTP adapter tests use loopback-only
test servers. It does not download videos or alter user data.

The functional contract is defined in the
[FR-04 specification](../../../docs/req/fr-04-multi-content-comparison.md).
Checks include natural-language answer display, per-source attribution and
answer retention when citation checks fail.

The acceptance mapping and executed regression results are recorded in
[D functional verification](../../../docs/evidence/d-fr-verification.md).
The canonical tests remain beside their production packages; this runner does
not duplicate or move the upstream tests.

These checks verify FR-04 and the integrations with A's invocation ledger,
B's retrieval/context bounds, and C's source verification. They do not measure
NFR-04 model accuracy by themselves. The [local self-evaluation runner](../../local/d-comparison/README.md) records this controlled regression as one NFR gate; its separate frozen 20-question, two-round real-model collection and author semantic/source scoring supply the quality metrics.
