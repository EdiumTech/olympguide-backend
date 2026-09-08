"""Verified Release -> transactional production catalogue import (no API token).

Uses the same rule/program projection as the desktop viewer. Generated SQL and
source documents remain outside Git. Run --help for preparation and SSH import.
"""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]


def literal(value):
    if value is None:
        return 'NULL'
    if isinstance(value, int):
        return str(value)
    if isinstance(value, (dict, list)):
        value = json.dumps(value, ensure_ascii=False, separators=(',', ':'))
    if '\x00' in value:
        raise ValueError('NUL in PostgreSQL text')
    return "'" + value.replace("'", "''") + "'"


def ref(table, column, key):
    return f'(SELECT {column} FROM olympguide.{table} WHERE catalog_key={literal(key)})'


def build_sql(catalog, manifest_sha, quantity_manifest_sha=None):
    """Upsert identities; replace only this admission year's rules atomically."""
    c = catalog
    year = c.bootstrap['admission_year']
    programs = {k: p for k, p in c.programs.items() if p['kind'] != 'group' and not p['aggregate']}
    olympiads = {}
    rule_keys = {}
    from catalog import normalized, identity
    for r in c.rules:
        v = r['values']
        key = identity(r['category'], normalized(v['olympiad_name']), normalized(v['olympiad_profile']))
        rule_keys[r['id']] = key
        o = olympiads.setdefault(key, dict(name=v['olympiad_name'], profile=v['olympiad_profile'],
                                         category=r['category'], levels=set(), url=r['source_url']))
        level = {'I': 1, 'II': 2, 'III': 3, '1': 1, '2': 2, '3': 3}.get(v.get('olympiad_level'), 0)
        o['levels'].add(level)
    stats = dict(admission_year=year, collected_on=c.bootstrap['collected_on'],
                 universities=len(c.universities), units=len(c.units), programs=len(programs),
                 fields=len(c.fields), olympiad_profiles=len(olympiads), rules=len(c.rules), sources=len(c.sources))
    if c.quantity_data:
        stats['quantities'] = dict(manifest_sha256=quantity_manifest_sha, coverage=c.quantity_data['coverage'])
    sql = ['BEGIN;', 'SET LOCAL standard_conforming_strings=on;', "SET LOCAL lock_timeout='15s';",
           'SELECT pg_advisory_xact_lock(2026, 7891);']
    def insert(table, data, conflict=None, update=None):
        statement = f"INSERT INTO olympguide.{table} ({','.join(data)}) VALUES ({','.join(data.values())})"
        if conflict:
            statement += f' ON CONFLICT ({conflict}) DO '
            statement += 'UPDATE SET ' + ','.join(f'{k}=EXCLUDED.{k}' for k in update) if update else 'NOTHING'
        sql.append(statement + ';')
    insert('admission_release', dict(admission_year=str(year), manifest_sha256=literal(manifest_sha), payload=literal(stats)),
           'admission_year', ['manifest_sha256', 'payload'])
    sql += [f'DELETE FROM olympguide.admission_program_rule WHERE admission_year={year};',
            f'DELETE FROM olympguide.admission_rule WHERE admission_year={year};',
            f'DELETE FROM olympguide.admission_source WHERE admission_year={year};']
    for key, u in sorted(c.universities.items()):
        region = 'Санкт-Петербург и Ленинградская область' if key == 'itmo' else 'Москва и Московская область'
        data = dict(catalog_key=literal(key), name=literal(u['name']), short_name=literal(u['short_name']),
                    site=literal(u['site']), description=literal(f"Приём {year}. {u['scope']}"),
                    region_id=f'(SELECT region_id FROM olympguide.region WHERE name={literal(region)})')
        insert('university', data, 'catalog_key', ['name', 'short_name', 'site', 'description', 'region_id'])
    # Import parents before departments; verified directory relationships are retained.
    for key, u in sorted(c.units.items(), key=lambda kv: (bool(kv[1]['parent_id']), kv[0])):
        data = dict(catalog_key=literal(key), name=literal(u['name']),
                    university_id=ref('university', 'university_id', u['university_id']),
                    description=literal(u.get('url', '')), unit_type=literal(u['type']),
                    parent_id=ref('faculty', 'faculty_id', u['parent_id']) if u['parent_id'] else 'NULL')
        insert('faculty', data, 'catalog_key', ['name', 'university_id', 'description', 'parent_id', 'unit_type'])
    for code, f in sorted(c.fields.items()):
        group = code[:2] + '.00.00'
        insert('group_of_fields', dict(name=literal(f['group']), code=literal(group)), 'code')
        insert('field_of_study', dict(name=literal(f['name']), code=literal(code), degree=literal(f['degree']),
               group_id=f'(SELECT group_id FROM olympguide.group_of_fields WHERE code={literal(group)})'), 'code')
    for key, p in sorted(programs.items()):
        units = p['unit_ids']
        data = dict(catalog_key=literal(key), name=literal(p['name']),
                    university_id=ref('university', 'university_id', p['university_id']),
                    faculty_id=ref('faculty', 'faculty_id', units[0]) if units else 'NULL',
                    field_id=f'(SELECT field_id FROM olympguide.field_of_study WHERE code={literal(p["field_id"])})',
                    budget_places=literal(p.get('budget_places')), paid_places=literal(p.get('paid_places')), cost=literal(p.get('cost')),
                    link=literal(p.get('program_url') or p.get('source_url') or c.universities[p['university_id']]['site']),
                    admission_metadata=literal(dict(p, admission_year=year, places_known=p.get('places_known', False), cost_known=p.get('cost_known', False), subjects_known=False)))
        insert('educational_program', data, 'catalog_key', ['name', 'university_id', 'faculty_id', 'field_id', 'link', 'admission_metadata', 'budget_places', 'paid_places', 'cost'])
        pid = ref('educational_program', 'program_id', key)
        sql.append(f'DELETE FROM olympguide.program_faculty WHERE program_id={pid};')
        for unit in sorted(set(units + p['department_ids'])):
            insert('program_faculty', dict(program_id=pid, faculty_id=ref('faculty', 'faculty_id', unit)))
    for key, o in sorted(olympiads.items()):
        level = next(iter(o['levels'])) if len(o['levels']) == 1 else 0
        data = dict(catalog_key=literal(key), name=literal(o['name']), profile=literal(o['profile']),
                    category=literal(o['category']), admission_year=str(year), level=str(level),
                    link=literal(o['url']), description=literal(f'Условия приёма {year}; уровень и год олимпиады уточняются в каждой строке правил.'))
        insert('olympiad', data, 'catalog_key', ['name', 'profile', 'category', 'admission_year', 'level', 'link', 'description'])
    for key, s in sorted(c.sources.items()):
        insert('admission_source', dict(admission_year=str(year), id=literal(key), payload=literal(s)))
    links = 0
    for r in c.rules:
        payload = c.public_rule(r)
        # Public source references point to the original official document.
        payload['source'].pop('local_url', None)
        insert('admission_rule', dict(admission_year=str(year), id=literal(r['id']),
               university_id=ref('university', 'university_id', r['university_id']),
               olympiad_id=ref('olympiad', 'olympiad_id', rule_keys[r['id']]), source_id=literal(r['source_id']),
               category=literal(r['category']), payload=literal(payload)))
        for pid in sorted(set(r['program_ids'] + r['related_program_ids']) & programs.keys()):
            relation = 'program_conditions' if pid in r['program_ids'] else programs[pid].get('rule_relation', 'school_conditions')
            if relation not in ('program_conditions', 'field_conditions', 'school_conditions'):
                raise ValueError(f'Unknown relation: {relation}')
            insert('admission_program_rule', dict(admission_year=str(year), rule_id=literal(r['id']),
                   program_id=ref('educational_program', 'program_id', pid), relation=literal(relation)))
            links += 1
    sql.append(f"DO $$ BEGIN IF (SELECT count(*) FROM olympguide.admission_rule WHERE admission_year={year}) <> {len(c.rules)} "
               f"OR (SELECT count(*) FROM olympguide.admission_program_rule WHERE admission_year={year}) <> {links} "
               "THEN RAISE EXCEPTION 'Catalogue import count mismatch'; END IF; END $$;")
    sql += ['COMMIT;']
    stats['program_rule_links'] = links
    return '\n'.join(sql) + '\n', stats


def build_quantity_sql(catalog, manifest_sha):
    if not catalog.quantity_data:
        raise ValueError('Verified quantity supplement is required')
    records = catalog.quantity_data['programs']
    rows = []
    for q in records:
        metadata = {k: v for k, v in q.items() if k not in ('id', 'university_id', 'field_id', 'name')}
        metadata['quantity_release'] = catalog.programs[q['id']]['quantity_release']
        rows.append('(' + ','.join(literal(v) for v in (q['id'], q['university_id'], q['field_id'],
                     q['budget_places'], q['paid_places'], q['cost'], metadata)) + ')')
    sql = ['BEGIN;', 'SET LOCAL standard_conforming_strings=on;', "SET LOCAL lock_timeout='15s';",
           'SELECT pg_advisory_xact_lock(2026, 7891);',
           'CREATE TEMP TABLE quantity_import (catalog_key text PRIMARY KEY, university_key text, field_code text, budget_places smallint, paid_places smallint, cost integer, metadata jsonb) ON COMMIT DROP;',
           'INSERT INTO quantity_import VALUES ' + ',\n'.join(rows) + ';',
           "DO $$ BEGIN IF (SELECT count(*) FROM quantity_import q JOIN olympguide.educational_program p USING(catalog_key) JOIN olympguide.university u USING(university_id) JOIN olympguide.field_of_study f USING(field_id) WHERE u.catalog_key=q.university_key AND f.code=q.field_code) <> " + str(len(records)) + " THEN RAISE EXCEPTION 'Quantity/program identity mismatch'; END IF; END $$;",
           "UPDATE olympguide.educational_program p SET budget_places=q.budget_places, paid_places=q.paid_places, cost=q.cost, admission_metadata=COALESCE(p.admission_metadata,'{}'::jsonb)||q.metadata FROM quantity_import q WHERE p.catalog_key=q.catalog_key;",
           'UPDATE olympguide.admission_release SET payload=payload||' + literal(dict(quantities=dict(manifest_sha256=manifest_sha, coverage=catalog.quantity_data['coverage']))) + '::jsonb WHERE admission_year=2026;',
           'COMMIT;']
    return '\n'.join(sql) + '\n', dict(programs=len(records), coverage=catalog.quantity_data['coverage'])


def prepare(static_repo, *, quantities_only=False):
    static_repo = static_repo.resolve()
    sys.path.insert(0, str(static_repo / 'data_loader'))
    sys.path.insert(0, str(static_repo / 'web'))
    from admissions.download import install
    from admissions.loader import validate
    from catalog import Catalog
    from admissions.quantity_catalog import download as download_quantities, MANIFEST as QUANTITY_MANIFEST
    manifest = static_repo / 'data_loader/admissions/releases/2026.json'
    snapshot = static_repo / 'data_loader/admissions/snapshots/2026'
    install(manifest_path=manifest, target=snapshot)
    validate(json.loads((snapshot / 'catalog.json').read_text(encoding='utf-8')), snapshot)
    download_quantities()
    catalog = Catalog(snapshot / 'catalog.json')
    if quantities_only:
        return build_quantity_sql(catalog, hashlib.sha256(QUANTITY_MANIFEST.read_bytes()).hexdigest())
    return build_sql(catalog, hashlib.sha256(manifest.read_bytes()).hexdigest(),
                     hashlib.sha256(QUANTITY_MANIFEST.read_bytes()).hexdigest())


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--static-repo', type=Path, required=True)
    parser.add_argument('--output', type=Path, default=ROOT / '.private/admissions-2026.sql')
    parser.add_argument('--host', help='Import over pinned SSH after backup; omit to only generate SQL')
    parser.add_argument('--quantities-only', action='store_true', help='Update tuition/places and provenance without replacing the olympiad catalog')
    args = parser.parse_args()
    sql, stats = prepare(args.static_repo, quantities_only=args.quantities_only)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(sql, encoding='utf-8')
    print(json.dumps(stats, ensure_ascii=True))
    print(f'Generated {args.output.stat().st_size:,} bytes of SQL outside the source tree.')
    if args.host:
        import ipaddress
        ipaddress.IPv4Address(args.host)
        from deploy import ssh_options
        ssh = ['ssh', *ssh_options(ROOT / '.private/deploy_ed25519', ROOT / '.private/known_hosts'), f'deploy@{args.host}']
        subprocess.run(ssh + ['sudo bash /srv/olympguide/current/deploy/backup.sh'], check=True)
        command = ('sudo docker compose --env-file /srv/olympguide/secrets/backend.env '
                   '-f /srv/olympguide/current/deploy/compose.yaml exec -T db '
                   'psql -X -q -v ON_ERROR_STOP=1 -U olympguide -d olympguide')
        # SQL is streamed through SSH; credentials never enter arguments or logs.
        with args.output.open('rb') as stream:
            subprocess.run(ssh + [command], stdin=stream, check=True)
        print('Committed catalogue import. Verify public API before reporting success.')


if __name__ == '__main__':
    main()
