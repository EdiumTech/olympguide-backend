"""Read-only checks of the deployed catalogue, including lossless pagination."""
import argparse
from concurrent.futures import ThreadPoolExecutor
import json
from pathlib import Path
import urllib.request


def verify(base, snapshot=None):
    def get(path):
        # Windows Internet Options may configure a separate legacy system proxy.
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        with opener.open(base.rstrip('/') + path, timeout=40) as response:
            return json.load(response)

    status = get('/admissions')[0]['catalogue']
    universities = get('/universities')
    assert len(universities) == status['universities'] == 7
    assert len(get('/olympiads')) == status['olympiad_profiles'] == 610
    programs = {}
    for university in universities:
        uid = university['university_id']
        units = get(f'/university/{uid}/faculties')
        tree = get(f'/university/{uid}/programs/by-faculty')
        fields = get(f'/university/{uid}/programs/by-field')
        ps = {p['program_id']: p for group in tree for p in group['programs']}
        assert ps and units and fields
        for program in ps.values():
            metadata = program.get('admission_metadata', {})
            if metadata.get('places_known') is False:
                assert program['budget_places'] is None and program['paid_places'] is None
            if metadata.get('cost_known') is False:
                assert program['cost'] is None
        programs.update(ps)
        count = get(f'/admissions/rules?university_id={uid}&limit=1')['total']
        assert count > 0
        for pid, program in ps.items():
            if program['admission_metadata'].get('rule_count', 0) == 0:
                continue
            detail = get(f'/program/{pid}/')
            assert detail['program_id'] == pid
            assert all(detail[key] == program[key] for key in ('budget_places', 'paid_places', 'cost'))
            benefits = get(f'/program/{pid}/benefits')
            assert benefits
            info = benefits[0]['benefits'][0]
            assert info['admission_rule']['conditions'] and info['min_class'] is None
            oid = benefits[0]['olympiad']['olympiad_id']
            assert uid in [u['university_id'] for u in get(f'/olympiad/{oid}/universities')]
            assert get(f'/olympiad/{oid}/benefits?university_id={uid}')
            break
        else:
            raise AssertionError(f'No program with conditions for university {uid}')
        print(f"{university['short_name']}: {len(units)} units, {len(ps)} programs, {count} rules", flush=True)
    assert len(programs) == status['programs'] == 379
    source = None
    if snapshot:
        source = {r['id']: r for r in json.loads((snapshot / 'catalog.json').read_text(encoding='utf-8'))['rules']}
    seen = set()
    def page(offset):
        return get(f'/admissions/rules?limit=100&offset={offset}')
    with ThreadPoolExecutor(max_workers=3) as executor:
        for response in executor.map(page, range(0, status['rules'], 100)):
            assert response['total'] == status['rules']
            for item in response['items']:
                rule = item['rule']
                assert item['id'] not in seen
                seen.add(item['id'])
                if source is not None:
                    original = source[item['id']]
                    assert all(rule[key] == value for key, value in original.items()), item['id']
    assert len(seen) == status['rules'] == 7891
    assert get('/admissions/rules?offset=7891')['items'] == []
    assert get('/universities?search=nonexistent-university-verification') == []
    print(f'Public API verified: {len(programs)} programs, {len(seen)} complete source rules.', flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--base-url', default='https://api.olympguide.ru/api/v1')
    parser.add_argument('--snapshot', type=Path)
    args = parser.parse_args()
    verify(args.base_url, args.snapshot)
