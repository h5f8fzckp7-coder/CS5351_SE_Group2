#!/usr/bin/env python3
"""Verify a published D evaluation bundle; no network or score changes."""
import argparse
import csv
import hashlib
import io
import json
from pathlib import Path
import zipfile

def sha(data):
    return hashlib.sha256(data).hexdigest()

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--archive', type=Path, required=True)
    args = parser.parse_args()
    with zipfile.ZipFile(args.archive) as z:
        manifest = json.loads(z.read('archive-manifest.json'))
        listed = set()
        entries = {e['path']: e for e in manifest['files']}
        for entry in manifest['files']:
            name = entry['path']
            if name in listed or name.startswith('/') or '..' in Path(name).parts:
                raise ValueError('Invalid or duplicate manifest path')
            listed.add(name)
            if sha(z.read(name)) != entry['archive_sha256']:
                raise ValueError('Artifact checksum mismatch: ' + name)
            if not entry['redacted'] and entry['original_sha256'] != entry['archive_sha256']:
                raise ValueError('Undisclosed artifact change: ' + name)
        if set(z.namelist()) != listed | {'archive-manifest.json'}:
            raise ValueError('Archive and manifest file sets differ')
        implementation = hashlib.sha256()
        for name in sorted(n for n in listed if n.startswith('implementation/')):
            original = name.removeprefix('implementation/')
            implementation.update(original.encode() + b'\0' + z.read(name) + b'\0')
        results = {}
        for profile in ('article_article', 'video_video', 'video_article'):
            freeze = json.loads(z.read(profile + '/freeze.json'))
            report = json.loads(z.read(profile + '/report.json'))
            frozen_sha = sha(z.read(profile + '/freeze.json'))
            source_manifest = json.loads(z.read(profile + '/manifest.json'))
            if (report['freeze_sha256'] != frozen_sha
                or freeze['manifest_sha256'] != sha(z.read(profile + '/manifest.json'))
                or freeze['cases_sha256'] != sha(z.read(profile + '/cases.csv'))):
                raise ValueError('Frozen manifest, cases or report mismatch')
            for cid, expected in freeze['source_sha256'].items():
                path = profile + '/' + source_manifest['source_snapshots'][cid]['file']
                if sha(z.read(path)) != expected:
                    raise ValueError('Frozen original-text checksum mismatch')
            regression = json.loads(z.read(profile + '/regression.json'))
            regression_pass = (regression['exit_code'] == 0
                and regression['freeze_sha256'] == frozen_sha
                and regression['log_sha256'] == entries[profile + '/regression.log']['original_sha256'])
            if not regression_pass or report['controlled_regression'] != 'passed':
                raise ValueError('Controlled-regression provenance mismatch')
            expected_responses = {f'run-{r}/D-{i:02}.json' for r in (1, 2) for i in range(1, 21)}
            if set(report['response_sha256']) != expected_responses:
                raise ValueError('Report must retain all 40 response hashes')
            if implementation.hexdigest() != freeze['implementation_sha256']:
                raise ValueError('Included implementation does not match frozen code')
            for path, expected in report['response_sha256'].items():
                if sha(z.read(profile + '/' + path)) != expected:
                    raise ValueError('Response hash mismatch')
            data = z.read(profile + '/scores.csv')
            if sha(data) != report['scores_sha256']:
                raise ValueError('Score hash mismatch')
            rows = list(csv.DictReader(io.StringIO(data.decode())))
            expected = {(str(r), f'D-{i:02}') for r in (1, 2) for i in range(1, 21)}
            if len(rows) != 40 or {(r['run'], r['case_id']) for r in rows} != expected:
                raise ValueError('Missing or duplicate score rows')
            for run in ('1', '2'):
                selected = [r for r in rows if r['run'] == run]
                for row in selected:
                    if not row['notes'].strip() or any(row[k] not in ('0', '1') for k in ('answer_correct', 'source_attribution_correct')):
                        raise ValueError('Incomplete semantic grading')
                for key in ('answer_correct', 'source_attribution_correct'):
                    if sum(int(r[key]) for r in selected) != report['runs'][run][key]:
                        raise ValueError('Score totals mismatch')
            quality_pass = all(report['runs'][run][key] >= 16 for run in ('1', '2') for key in ('answer_correct', 'source_attribution_correct'))
            if report['outcome'] != ('PASS' if quality_pass and regression_pass else 'FAIL'):
                raise ValueError('Reported threshold outcome mismatch')
            results[profile] = {'outcome': report['outcome'], 'runs': report['runs']}
    print(json.dumps({'bundle_integrity': 'PASS', 'profiles': results,
        'scope': 'Integrity and recorded arithmetic only; semantic grades are assisted self-evaluation.'}, indent=2))

if __name__ == '__main__':
    main()
