#!/usr/bin/env python3
"""Check sanitized session chronology and hashes without revealing conversation text."""
import csv
import gzip
import hashlib
import io
import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[3]
FOLDER = ROOT / 'docs/evidence/d-session-export'

def main():
    manifest = json.loads((FOLDER / 'manifest.json').read_text())
    for name, expected in manifest['files'].items():
        if hashlib.sha256((FOLDER / name).read_bytes()).hexdigest() != expected:
            raise ValueError('Export artifact checksum mismatch: ' + name)
    forbidden = [r'\bsk-[A-Za-z0-9_-]{16,}', r'Bearer\s+[A-Za-z0-9_.-]{16,}',
                 r'data:image/[^;,\s]+;base64,[A-Za-z0-9+/=]{50,}', r'/(?:Users|home)/']
    count, user_records = 0, set()
    with gzip.open(FOLDER / 'conversation.sanitized.jsonl.gz', 'rt', encoding='utf-8') as handle:
        for line in handle:
            count += 1
            record = json.loads(line)
            if record['original_record'] != count or not re.fullmatch('[0-9a-f]{64}', record['original_line_sha256']):
                raise ValueError('Missing or reordered original record')
            if any(re.search(pattern, line) for pattern in forbidden):
                raise ValueError('Sensitive pattern needs review at original record ' + str(count))
            payload = record.get('payload', {})
            if record['type'] == 'response_item' and payload.get('role') == 'user':
                user_records.add(count)
    rows = list(csv.DictReader(io.StringIO((FOLDER / 'student-interventions.csv').read_text())))
    indexed = [int(row['original_record']) for row in rows]
    if len(indexed) != len(set(indexed)) or set(indexed) != user_records:
        raise ValueError('User intervention index does not match actual user messages')
    if count != manifest['source_records'] or len(rows) != manifest['user_interventions']:
        raise ValueError('Export record counts mismatch')
    print(f'Verified {count} chronological records, {len(rows)} actual user-message annotations and artifact hashes; this does not certify course acceptance or independent approvals.')

if __name__ == '__main__':
    main()
