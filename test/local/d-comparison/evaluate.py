#!/usr/bin/env python3
"""Local NFR-04 self-evaluation. Only collect calls the configured model via a running local app."""
import argparse
import csv
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timezone

ROOT = Path(__file__).resolve().parents[3]
CASE_FIELDS = ["case_id", "question", "source_ids", "expected_answer_points", "expected_source_points"]
SCORE_FIELDS = ["run", "case_id", "answer_correct", "source_attribution_correct", "notes"]
CASE_IDS = [f"D-{i:02}" for i in range(1, 21)]


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def write_json(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def read_json(path):
    return json.loads(path.read_text(encoding="utf-8"))


def read_csv(path, fields):
    with path.open(encoding="utf-8-sig", newline="") as handle:
        reader = csv.DictReader(handle)
        if reader.fieldnames != fields:
            raise ValueError(f"Unexpected CSV columns: {path}")
        rows = list(reader)
    if any(None in row or any(v is None for v in row.values()) for row in rows):
        raise ValueError(f"Malformed CSV row: {path}")
    return rows


def code_hash():
    paths = [*ROOT.glob("src/internal/**/*.go"), *ROOT.glob("src/web/src/**/*")]
    paths += [ROOT / "src/go.mod", ROOT / "src/go.sum", ROOT / "src/web/package-lock.json"]
    h = hashlib.sha256()
    for path in sorted(p for p in paths if p.is_file() and not p.name.endswith("_test.go")):
        h.update(str(path.relative_to(ROOT)).encode())
        h.update(b"\0")
        h.update(path.read_bytes())
        h.update(b"\0")
    return h.hexdigest()


def initialize(folder):
    folder.mkdir(parents=True, exist_ok=False)
    for run in (1, 2):
        (folder / f"run-{run}").mkdir()
    (folder / "snapshots").mkdir()
    write_json(folder / "manifest.json", {
        "author": "", "model": "", "settings_revision": None,
        "configuration": {"maxProviderCalls": 3, "maxChars": 120000},
        "source_snapshots": {
            "REPLACE_WITH_SAVED_CONTENT_ID_A": {
                "kind": "article", "version": "", "file": "snapshots/source-a.txt"},
            "REPLACE_WITH_SAVED_CONTENT_ID_B": {
                "kind": "article", "version": "", "file": "snapshots/source-b.txt"}},
        "grading": "author_self_evaluation"})
    with (folder / "cases.csv").open("w", encoding="utf-8", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=CASE_FIELDS)
        writer.writeheader()
        writer.writerows({"case_id": cid} for cid in CASE_IDS)
    with (folder / "scores.csv").open("w", encoding="utf-8", newline="") as handle:
        writer = csv.DictWriter(handle, fieldnames=SCORE_FIELDS)
        writer.writeheader()
        writer.writerows({"run": run, "case_id": cid} for run in (1, 2) for cid in CASE_IDS)
    print(f"Created unscored local evaluation templates: {folder}")


def freeze(folder):
    if (folder / "freeze.json").exists():
        raise ValueError("Already frozen; use a new evaluation directory for another dataset/configuration.")
    manifest = read_json(folder / "manifest.json")
    for field in ("author", "model"):
        if not isinstance(manifest.get(field), str) or not manifest[field].strip():
            raise ValueError(f"Fill manifest.{field} before freezing.")
    if type(manifest.get("settings_revision")) is not int or manifest["settings_revision"] < 1:
        raise ValueError("Record a positive saved settings revision.")
    if manifest.get("grading") != "author_self_evaluation":
        raise ValueError("This protocol must be labeled author_self_evaluation.")
    if not isinstance(manifest.get("configuration"), dict) or not manifest["configuration"]:
        raise ValueError("Record the actual generation/retrieval configuration.")
    for field, maximum in (("maxProviderCalls", 100), ("maxChars", 120000)):
        if field in manifest["configuration"]:
            value = manifest["configuration"][field]
            if type(value) is not int or not 1 <= value <= maximum:
                raise ValueError(f"Invalid frozen {field}.")
    sources = manifest.get("source_snapshots", {})
    if not isinstance(sources, dict) or len(sources) < 2:
        raise ValueError("Record at least two saved source snapshots.")
    snapshots = {}
    for cid, source in sources.items():
        if (not isinstance(cid, str) or cid.startswith("REPLACE_") or
                source.get("kind") not in ("article", "subtitle") or not source.get("version")):
            raise ValueError(f"Fill the source ID, kind and saved version: {cid}")
        path = (folder / source["file"]).resolve()
        if not path.is_file() or not path.stat().st_size:
            raise ValueError(f"Save nonempty original text: {path}")
        snapshots[cid] = digest(path)
    cases = read_csv(folder / "cases.csv", CASE_FIELDS)
    if [c["case_id"] for c in cases] != CASE_IDS:
        raise ValueError("cases.csv must contain D-01 through D-20 once, in order.")
    for case in cases:
        if any(not value.strip() for value in case.values()):
            raise ValueError(f"Pre-annotate every question and expected finding: {case['case_id']}")
        ids = case["source_ids"].split("|")
        if len(ids) not in (2, 3) or len(set(ids)) != len(ids) or not set(ids) <= set(sources):
            raise ValueError(f"Use two/three distinct recorded source IDs: {case['case_id']}")
    if any(any((folder / f"run-{run}").iterdir()) for run in (1, 2)):
        raise ValueError("Freeze expected findings before collecting any run responses.")
    write_json(folder / "freeze.json", {
        "frozen_at": datetime.now(timezone.utc).isoformat(),
        "manifest_sha256": digest(folder / "manifest.json"),
        "cases_sha256": digest(folder / "cases.csv"),
        "source_sha256": snapshots, "implementation_sha256": code_hash(),
        "cases": 20, "runs": 2, "minimum_correct_per_metric_per_run": 16})
    print("Frozen 20 pre-annotated cases and configuration; model calls started: 0.")


def verify_freeze(folder):
    frozen = read_json(folder / "freeze.json")
    manifest = read_json(folder / "manifest.json")
    for filename, key in (("manifest.json", "manifest_sha256"), ("cases.csv", "cases_sha256")):
        if digest(folder / filename) != frozen[key]:
            raise ValueError(f"Frozen {filename} changed; prepare a new evaluation and rerun both rounds.")
    if code_hash() != frozen["implementation_sha256"]:
        raise ValueError("Implementation changed since freezing; rerun in a new evaluation directory.")
    for cid, expected in frozen["source_sha256"].items():
        if digest(folder / manifest["source_snapshots"][cid]["file"]) != expected:
            raise ValueError(f"Frozen original text changed: {cid}")
    return frozen, manifest


def regression(folder):
    verify_freeze(folder)
    if (folder / "regression.json").exists() or (folder / "regression.log").exists():
        raise ValueError("Regression evidence already exists; preserve it and use a new evaluation directory.")
    command = ["sh", "test/offline/d-comparison/run.sh"]
    with (folder / "regression.log").open("wb") as output:
        completed = subprocess.run(command, cwd=ROOT, stdout=output, stderr=subprocess.STDOUT, check=False)
    write_json(folder / "regression.json", {
        "command": command, "exit_code": completed.returncode,
        "freeze_sha256": digest(folder / "freeze.json"),
        "log_sha256": digest(folder / "regression.log")})
    print(f"Local controlled regression exit code: {completed.returncode}; see regression.log")
    return completed.returncode


def collect(folder, run, api):
    """Explicitly requested real comparisons against an already running local app."""
    _, manifest = verify_freeze(folder)
    parsed = urllib.parse.urlsplit(api)
    if (parsed.scheme != "http" or parsed.hostname not in ("127.0.0.1", "localhost", "::1") or
            parsed.username or parsed.password or parsed.query or parsed.fragment):
        raise ValueError("Use an HTTP loopback application URL, for example http://127.0.0.1:7380/api.")
    if run not in (1, 2):
        raise ValueError("collect requires --run 1 or --run 2")
    if run == 2 and any(not (folder / f"run-1/{cid}.json").is_file() for cid in CASE_IDS):
        raise ValueError("Collect the complete first round before the repeat.")
    api = api.rstrip("/")
    with urllib.request.urlopen(api + "/ai/settings", timeout=10) as response:
        settings = json.load(response)
    if settings.get("revision") != manifest["settings_revision"] or settings.get("chatModel") != manifest["model"]:
        raise ValueError("Local saved conversation model/revision differs from the frozen configuration.")
    print("Collecting real comparisons through the configured model; existing responses will not be retried.", flush=True)
    for case in read_csv(folder / "cases.csv", CASE_FIELDS):
        path = folder / f"run-{run}/{case['case_id']}.json"
        if path.exists():
            print(f"Preserved existing response: {path.name}", flush=True)
            continue
        body = {"contentIds": case["source_ids"].split("|"), "question": case["question"],
                "revision": manifest["settings_revision"], "model": manifest["model"]}
        for field in ("maxProviderCalls", "maxChars"):
            if field in manifest["configuration"]:
                body[field] = manifest["configuration"][field]
        request = urllib.request.Request(api + "/ai/compare", data=json.dumps(body).encode("utf-8"),
                                         headers={"Content-Type": "application/json"})
        started = time.monotonic()
        # Retain an interrupted/unknown attempt instead of retrying it silently.
        write_json(path, {"status": "failed", "message": "Request started; no completed response recorded.",
                          "localEvaluation": {"httpStatus": 0, "run": run, "case_id": case["case_id"],
                                              "freeze_sha256": digest(folder / "freeze.json")}})
        try:
            try:
                response = urllib.request.urlopen(request, timeout=180)
            except urllib.error.HTTPError as error:
                response = error
            with response:
                status = response.code
                try:
                    result = json.load(response)
                except (ValueError, UnicodeError):
                    result = {"status": "failed", "message": "Unparseable local API response."}
            if not isinstance(result, dict):
                result = {"status": "failed", "message": "Unexpected local API response shape."}
        except (OSError, urllib.error.URLError) as error:
            status = 0
            result = {"status": "failed", "message": f"Local request failed: {type(error).__name__}"}
        result["localEvaluation"] = {"httpStatus": status, "run": run, "case_id": case["case_id"],
                                     "freeze_sha256": digest(folder / "freeze.json"),
                                     "elapsed_seconds": round(time.monotonic() - started, 3)}
        write_json(path, result)
        print(f"run {run} {case['case_id']}: HTTP {status}; raw response saved", flush=True)
    print("Collection complete. Original text and semantic findings still require author scoring.")


def score(folder):
    frozen, manifest = verify_freeze(folder)
    cases = {c["case_id"]: c for c in read_csv(folder / "cases.csv", CASE_FIELDS)}
    rows = read_csv(folder / "scores.csv", SCORE_FIELDS)
    expected = {(str(run), cid) for run in (1, 2) for cid in CASE_IDS}
    actual = [(r["run"], r["case_id"]) for r in rows]
    if len(actual) != 40 or len(set(actual)) != 40 or set(actual) != expected:
        raise ValueError("scores.csv must retain all 40 run/case rows exactly once.")
    totals = {str(run): {"answer_correct": 0, "source_attribution_correct": 0,
                         "denominator": 20, "unscored": 0} for run in (1, 2)}
    issues, artifacts = [], {}
    for row in rows:
        run, cid = row["run"], row["case_id"]
        name = f"run-{run}/{cid}.json"
        path = folder / name
        marks = {field: row[field] for field in ("answer_correct", "source_attribution_correct")}
        unscored = any(v not in ("0", "1") for v in marks.values()) or not row["notes"].strip()
        if unscored:
            issues.append(f"{run}/{cid}: both manual 0/1 scores and a grading rationale are required")
        answer_ok, source_ok = True, True
        try:
            body = read_json(path)
            result = body.get("result", body)
            if not isinstance(result, dict):
                raise ValueError("invalid result object")
            metadata = body.get("localEvaluation", {})
            if (metadata.get("freeze_sha256") != digest(folder / "freeze.json") or
                    metadata.get("run") != int(run) or metadata.get("case_id") != cid):
                raise ValueError("response was not collected for this frozen run/case")
            answer = result.get("answer", "")
            answer_ok = (metadata.get("httpStatus") == 200 and result.get("status") == "completed" and
                         result.get("model") == manifest["model"] and
                         isinstance(answer, str) and bool(answer.strip()) and
                         len(answer) <= 600 and "\n" not in answer and "\r" not in answer)
            ids = [source.get("contentId") for source in result.get("sources", [])]
            source_ok = ids == cases[cid]["source_ids"].split("|")
            artifacts[name] = digest(path)
        except (OSError, ValueError, TypeError, AttributeError) as error:
            answer_ok = source_ok = False
            issues.append(f"{run}/{cid}: missing or invalid raw response ({type(error).__name__})")
            unscored = True
        totals[run]["unscored"] += int(unscored)
        if not answer_ok:
            issues.append(f"{run}/{cid}: response/model/status/answer format failed; answer scores zero")
        if not source_ok:
            issues.append(f"{run}/{cid}: source identity/order failed; attribution scores zero")
        totals[run]["answer_correct"] += int(marks["answer_correct"] == "1" and answer_ok)
        totals[run]["source_attribution_correct"] += int(marks["source_attribution_correct"] == "1" and source_ok)
    regression_state = "missing"
    try:
        evidence = read_json(folder / "regression.json")
        valid = (evidence["freeze_sha256"] == digest(folder / "freeze.json") and
                 evidence["command"] == ["sh", "test/offline/d-comparison/run.sh"] and
                 evidence["log_sha256"] == digest(folder / "regression.log"))
        regression_state = "passed" if valid and evidence["exit_code"] == 0 else "failed"
    except (OSError, ValueError, KeyError, TypeError):
        pass
    incomplete = any(t["unscored"] for t in totals.values()) or regression_state == "missing"
    quality_pass = all(t[field] >= 16 for t in totals.values()
                       for field in ("answer_correct", "source_attribution_correct"))
    outcome = "INCOMPLETE" if incomplete else "PASS" if quality_pass and regression_state == "passed" else "FAIL"
    report = {"outcome": outcome, "grading": "author_self_evaluation", "author": manifest["author"],
              "model": manifest["model"], "runs": totals, "controlled_regression": regression_state,
              "quality_thresholds_met": quality_pass, "issues": issues,
              "freeze_sha256": digest(folder / "freeze.json"), "response_sha256": artifacts,
              "scores_sha256": digest(folder / "scores.csv"),
              "scope": "Local NFR-04 self-evaluation; not independent review or PR approval."}
    write_json(folder / "report.json", report)
    print(json.dumps({k: report[k] for k in ("outcome", "runs", "controlled_regression")}, ensure_ascii=False, indent=2))
    return 2 if incomplete else 0 if outcome == "PASS" else 1


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("init", "freeze", "collect", "regression", "score"))
    parser.add_argument("--dir", required=True, type=Path, help="New local evidence directory for init; existing directory otherwise.")
    parser.add_argument("--run", type=int, choices=(1, 2), help="Round to collect; each round has 20 sequential comparisons.")
    parser.add_argument("--api", default="http://127.0.0.1:7380/api", help="Already running local application API; collect only.")
    args = parser.parse_args()
    folder = args.dir.expanduser().resolve()
    try:
        if args.action == "collect":
            return collect(folder, args.run, args.api) or 0
        return {"init": initialize, "freeze": freeze, "regression": regression, "score": score}[args.action](folder) or 0
    except (OSError, ValueError, KeyError, TypeError) as error:
        print(f"Evaluation incomplete: {error}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
