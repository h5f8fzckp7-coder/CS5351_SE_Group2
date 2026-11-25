"""Scoring-tool checks use synthetic responses, never actual quality scores."""
import contextlib
import csv
import importlib.util
import io
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("local_evaluation", Path(__file__).with_name("evaluate.py"))
evaluation = importlib.util.module_from_spec(spec)
spec.loader.exec_module(evaluation)


class EvaluationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.folder = Path(self.temp.name) / "evidence"
        self.capture = contextlib.redirect_stdout(io.StringIO())
        self.capture.__enter__()
        self.addCleanup(self.capture.__exit__, None, None, None)
        evaluation.initialize(self.folder)
        manifest = evaluation.read_json(self.folder / "manifest.json")
        manifest.update(author="Synthetic test author", model="fixture-model", settings_revision=1)
        manifest["source_snapshots"] = {
            f"rss:{letter * 64}": {"kind": "article", "version": "1", "file": f"snapshots/{letter}.txt"}
            for letter in ("a", "b")}
        for source in manifest["source_snapshots"].values():
            (self.folder / source["file"]).write_text("Synthetic original passage.", encoding="utf-8")
        evaluation.write_json(self.folder / "manifest.json", manifest)
        self.ids = list(manifest["source_snapshots"])
        self.cases = [{"case_id": cid, "question": "Synthetic comparison?", "source_ids": "|".join(self.ids),
                       "expected_answer_points": "Synthetic equivalence.",
                       "expected_source_points": "Both originals support the same synthetic fact."}
                      for cid in evaluation.CASE_IDS]
        self.write_rows("cases.csv", evaluation.CASE_FIELDS, self.cases)

    def write_rows(self, name, fields, rows):
        with (self.folder / name).open("w", encoding="utf-8", newline="") as handle:
            writer = csv.DictWriter(handle, fieldnames=fields)
            writer.writeheader()
            writer.writerows(rows)

    def complete(self):
        evaluation.freeze(self.folder)
        self.rows = []
        for run in (1, 2):
            for cid in evaluation.CASE_IDS:
                evaluation.write_json(self.folder / f"run-{run}/{cid}.json", {
                    "status": "completed", "model": "fixture-model", "answer": "Synthetic answer.",
                    "sources": [{"contentId": source_id} for source_id in self.ids],
                    "localEvaluation": {"httpStatus": 200, "run": run, "case_id": cid,
                                        "freeze_sha256": evaluation.digest(self.folder / "freeze.json")}})
                self.rows.append({"run": run, "case_id": cid, "answer_correct": 1,
                                  "source_attribution_correct": 1, "notes": "Synthetic scoring-tool test only."})
        self.write_rows("scores.csv", evaluation.SCORE_FIELDS, self.rows)
        with patch.object(evaluation.subprocess, "run", return_value=SimpleNamespace(returncode=0)) as run:
            evaluation.regression(self.folder)
            self.assertEqual(run.call_args.args[0], ["sh", "test/offline/d-comparison/run.sh"])

    def test_blank_template_cannot_freeze(self):
        self.cases[0]["expected_answer_points"] = ""
        self.write_rows("cases.csv", evaluation.CASE_FIELDS, self.cases)
        with self.assertRaises(ValueError):
            evaluation.freeze(self.folder)

    def test_duplicate_cases_cannot_freeze(self):
        self.cases[-1] = self.cases[0]
        self.write_rows("cases.csv", evaluation.CASE_FIELDS, self.cases)
        with self.assertRaises(ValueError):
            evaluation.freeze(self.folder)

    def test_complete_two_runs_and_regression(self):
        self.complete()
        self.assertEqual(evaluation.score(self.folder), 0)
        report = evaluation.read_json(self.folder / "report.json")
        self.assertEqual(report["grading"], "author_self_evaluation")
        self.assertEqual(report["runs"]["1"]["answer_correct"], 20)

    def test_each_run_must_pass_without_pooling(self):
        self.complete()
        for row in self.rows[:5]:
            row["answer_correct"] = 0
        self.write_rows("scores.csv", evaluation.SCORE_FIELDS, self.rows)
        self.assertEqual(evaluation.score(self.folder), 1)
        report = evaluation.read_json(self.folder / "report.json")
        self.assertEqual(report["runs"]["1"]["answer_correct"], 15)
        self.assertEqual(report["runs"]["2"]["answer_correct"], 20)

    def test_missing_raw_response_is_incomplete(self):
        self.complete()
        (self.folder / "run-1/D-01.json").unlink()
        self.assertEqual(evaluation.score(self.folder), 2)
        self.assertEqual(evaluation.read_json(self.folder / "report.json")["runs"]["1"]["denominator"], 20)

    def test_blank_scoring_rationale_is_incomplete(self):
        self.complete()
        self.rows[0]["notes"] = ""
        self.write_rows("scores.csv", evaluation.SCORE_FIELDS, self.rows)
        self.assertEqual(evaluation.score(self.folder), 2)

    def test_source_swap_cannot_receive_attribution_credit(self):
        self.complete()
        for cid in evaluation.CASE_IDS[:5]:
            path = self.folder / f"run-1/{cid}.json"
            body = evaluation.read_json(path)
            body["sources"].reverse()
            evaluation.write_json(path, body)
        self.assertEqual(evaluation.score(self.folder), 1)
        self.assertEqual(evaluation.read_json(self.folder / "report.json")["runs"]["1"]["source_attribution_correct"], 15)

    def test_frozen_material_change_rejected(self):
        self.complete()
        (self.folder / "snapshots/a.txt").write_text("Changed source.", encoding="utf-8")
        with self.assertRaises(ValueError):
            evaluation.score(self.folder)

    def test_missing_regression_cannot_pass(self):
        self.complete()
        (self.folder / "regression.json").unlink()
        self.assertEqual(evaluation.score(self.folder), 2)

    def test_recorded_regression_failure_blocks_acceptance(self):
        self.complete()
        path = self.folder / "regression.json"
        record = evaluation.read_json(path)
        record["exit_code"] = 1
        evaluation.write_json(path, record)
        self.assertEqual(evaluation.score(self.folder), 1)

    def test_duplicate_score_row_rejected(self):
        self.complete()
        self.rows[-1] = self.rows[0]
        self.write_rows("scores.csv", evaluation.SCORE_FIELDS, self.rows)
        with self.assertRaises(ValueError):
            evaluation.score(self.folder)

    def test_collection_before_freeze_rejected(self):
        evaluation.write_json(self.folder / "run-1/D-01.json", {"answer": "Premature output."})
        with self.assertRaises(ValueError):
            evaluation.freeze(self.folder)

    def test_collection_rejects_nonlocal_endpoint_without_network(self):
        evaluation.freeze(self.folder)
        with patch.object(evaluation.urllib.request, "urlopen") as open_url:
            with self.assertRaises(ValueError):
                evaluation.collect(self.folder, 1, "https://example.com/api")
            open_url.assert_not_called()

    def test_collection_preserves_all_existing_responses(self):
        self.complete()
        settings = io.BytesIO(b'{"revision":1,"chatModel":"fixture-model"}')
        with patch.object(evaluation.urllib.request, "urlopen", return_value=settings) as open_url:
            evaluation.collect(self.folder, 1, "http://127.0.0.1:7380/api")
            self.assertEqual(open_url.call_count, 1)  # Settings only; zero new model requests.

    def test_collection_saves_twenty_real_shape_responses_without_gold(self):
        evaluation.freeze(self.folder)
        responses = [io.BytesIO(b'{"revision":1,"chatModel":"fixture-model"}')]
        for _ in evaluation.CASE_IDS:
            response = io.BytesIO(b'{"status":"completed","model":"fixture-model","answer":"Test."}')
            response.code = 200
            responses.append(response)
        with patch.object(evaluation.urllib.request, "urlopen", side_effect=responses) as open_url:
            evaluation.collect(self.folder, 1, "http://127.0.0.1:7380/api")
            self.assertEqual(open_url.call_count, 21)
            for call in open_url.call_args_list[1:]:
                self.assertNotIn(b"expected_answer", call.args[0].data)
                self.assertNotIn(b"Synthetic equivalence", call.args[0].data)
        self.assertEqual(len(list((self.folder / "run-1").glob("*.json"))), 20)

    def test_response_from_other_freeze_is_incomplete(self):
        self.complete()
        path = self.folder / "run-1/D-01.json"
        body = evaluation.read_json(path)
        body["localEvaluation"]["freeze_sha256"] = "another freeze"
        evaluation.write_json(path, body)
        self.assertEqual(evaluation.score(self.folder), 2)


if __name__ == "__main__":
    unittest.main()
