#!/usr/bin/env python3
"""Verify the published historical bundle without writing scores or calling a model."""
import csv
import hashlib
import io
import json
import subprocess
import tarfile
from pathlib import Path
import zipfile

ROOT = Path(__file__).resolve().parents[3]
ARCHIVE = ROOT / 'docs/evidence/d-nfr-full-20261125-002.zip'
EXPECTED = '1c6ddec05c28150a83996e31114c3c93156b8076bf66f1ffdc8f6f71d6ca4c52'

def sha(data):
    return hashlib.sha256(data).hexdigest()

def main():
    if sha(ARCHIVE.read_bytes()) != EXPECTED:
        raise ValueError('Archive checksum mismatch')
    # Verify the historical implementation against committed bytes, not today's worktree.
    commit = subprocess.check_output(['git', 'archive', '99c38dd'], cwd=ROOT)
    h = hashlib.sha256()
    with tarfile.open(fileobj=io.BytesIO(commit)) as tree:
        paths = []
        for member in tree.getmembers():
            name = member.name
            if member.isfile() and (
                (name.startswith('src/internal/') and name.endswith('.go') and not name.endswith('_test.go'))
                or name.startswith('src/web/src/')
                or name in ('src/go.mod', 'src/go.sum', 'src/web/package-lock.json')
            ):
                paths.append((name, tree.extractfile(member).read()))
        for name, data in sorted(paths):
            h.update(name.encode() + b'\0' + data + b'\0')
    with zipfile.ZipFile(ARCHIVE) as z:
        manifest = json.loads(z.read('archive-manifest.json'))
        for entry in manifest['files']:
            if sha(z.read(entry['path'])) != entry['archive_sha256']:
                raise ValueError('Artifact checksum mismatch: ' + entry['path'])
            if not entry['redacted'] and entry['archive_sha256'] != entry['original_sha256']:
                raise ValueError('Unexpected original-byte mismatch')
        for profile in ('article_article', 'video_video', 'video_article'):
            frozen = json.loads(z.read(profile + '/freeze.json'))
            if h.hexdigest() != frozen['implementation_sha256']:
                raise ValueError('Historical implementation freeze mismatch')
            report = json.loads(z.read(profile + '/report.json'))
            for path, expected in report['response_sha256'].items():
                if sha(z.read(profile + '/' + path)) != expected:
                    raise ValueError('Response checksum mismatch: ' + path)
            scores = z.read(profile + '/scores.csv')
            if sha(scores) != report['scores_sha256']:
                raise ValueError('Score checksum mismatch')
            rows = list(csv.DictReader(io.StringIO(scores.decode())))
            if len(rows) != 40 or len({(r['run'], r['case_id']) for r in rows}) != 40:
                raise ValueError('Missing or duplicate score rows')
            totals = {}
            for run in ('1', '2'):
                selected = [r for r in rows if r['run'] == run]
                if len(selected) != 20 or any(not r['notes'].strip() for r in selected):
                    raise ValueError('Incomplete grading')
                totals[run] = {k: sum(int(r[k]) for r in selected) for k in ('answer_correct', 'source_attribution_correct')}
                for k, value in totals[run].items():
                    if value != report['runs'][run][k]:
                        raise ValueError('Reported totals mismatch')
            print(profile, json.dumps(totals, sort_keys=True))
    print('Verified archive bytes, 120 response hashes, 120 score rows and recorded totals; semantic grades remain assisted self-evaluation.')

if __name__ == '__main__':
    main()
