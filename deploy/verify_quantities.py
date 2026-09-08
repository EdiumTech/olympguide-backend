"""Compare all live list/detail quantity values and provenance with the verified Release."""
import argparse
from concurrent.futures import ThreadPoolExecutor
import json
from pathlib import Path
import sys
import urllib.request


def verify(static_repo, base):
    sys.path.insert(0, str(static_repo.resolve() / 'data_loader'))
    from admissions.quantity_catalog import load
    source = load()
    expected = {p['id']: p for p in source['programs']}

    def get(path):
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        with opener.open(base.rstrip('/') + path, timeout=40) as response:
            return json.load(response)

    def check(p):
        metadata = p['admission_metadata']
        q = expected[metadata['id']]
        for key in ('budget_places', 'paid_places', 'cost'):
            assert p[key] == q[key], (p['program_id'], key, p[key], q[key])
        for key in ('cost_period', 'places_known', 'cost_known', 'study_form', 'places_note', 'quantity_notes', 'quantity_evidence'):
            assert metadata[key] == q[key], (p['program_id'], key)
        return metadata['id']

    programs = {}
    for u in get('/universities'):
        for grouping in ('by-faculty', 'by-field'):
            for group in get(f'/university/{u["university_id"]}/programs/{grouping}'):
                for p in group['programs']:
                    check(p)
                    programs[p['program_id']] = p
    assert {check(p) for p in programs.values()} == expected.keys()
    with ThreadPoolExecutor(max_workers=3) as pool:
        for p in pool.map(lambda pid: get(f'/program/{pid}/'), programs): check(p)
    print(json.dumps(dict(verified_programs=len(programs), coverage=source['coverage']), ensure_ascii=False))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--static-repo', type=Path, required=True)
    parser.add_argument('--base-url', default='https://api.olympguide.ru/api/v1')
    args = parser.parse_args()
    verify(args.static_repo, args.base_url)
