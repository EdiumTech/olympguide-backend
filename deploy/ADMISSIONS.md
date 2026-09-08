# Admissions catalogue 2026

The application database imports the verified dataset published in the
[2026 Release](https://github.com/EdiumTech/olympguide-static/releases/tag/admissions-2026-20260906).
Source PDFs, HTML, JSON and generated SQL remain outside Git.

Deploy the API/migrations first, then run from this repository:

```powershell
python -X utf8 deploy/import_admissions.py --static-repo ../olympguide-static
python -X utf8 deploy/import_admissions.py --static-repo ../olympguide-static --host 158.160.150.215
```

The static checkout must include the Release manifest, download module and web
catalogue projection (PR35). The loader verifies every pinned file, generates SQL
in `.private`, and takes a PostgreSQL backup before sending SQL over pinned SSH.
An advisory lock and a single transaction cover the import. Repeating the same
import preserves numeric entity IDs and replaces only the selected year's rules.
Other years, user accounts, favourites and legacy benefits are not deleted.

Coverage: seven universities; 206 organisational units (91 main units and 115
departments); 378 verified directory programs plus one field mentioned only in
the MEPHI benefit document; 109 field codes; 610 olympiad name/profile/category
combinations; 7,891 source rules; 163 sources including organisation directories.
The 127 olympiad names in the viewer become separate API entries by profile.
The old seeded olympiads remain addressable by ID; the main list shows the
imported year when a release is present. No fuzzy matching to old diploma IDs is
performed.

## Public API

- `/api/v1/universities`: seven populated university records with numeric IDs.
- Existing university, field, program, faculty and olympiad routes use the same
  IDs. Program trees include verified many-to-many faculty affiliations.
- `/api/v1/admissions`: imported version, counts and import timestamp.
- `/api/v1/admissions/rules`: **all** source rules, including university-wide and
  school-wide scopes that cannot be assigned to an individual program.
  Query parameters: `admission_year` (2026), `university_id`, `olympiad_id`,
  `program_id`, `category`, `benefit` (`bvi` or `100_points`), `q`, `limit` (1–100)
  and `offset`. Response: `total`, `items`, `limit`, `offset`, `admission_year`.
  Each item exposes numeric API IDs separately from the original `rule` JSON;
  IDs inside the original JSON are the source catalogue's stable string keys.
- Existing `/program/:id/benefits` and `/olympiad/:id/benefits` include source
  conditions for associated programs. Each such benefit has `admission_rule`
  containing the full original values, conditions, year and official source URL,
  and `source_relation`: `program_conditions`, `field_conditions` or
  `school_conditions`. School membership is a reference to school conditions,
  not an assertion that every condition applies to every competition group.

For source rules, `min_class` and `min_diploma_level` are **null**: a single
minimum would lose exceptions and alternatives. `is_bvi` only describes whether
the original row mentions BVI; clients must render `admission_rule.benefit_types`
and `values.benefit` together with all conditions. Updated iOS models and the
scrollable benefit sheet support these fields. An older app build must be updated.

Automatic matching by a user's diploma, and numerical class/diploma filters,
continue to use legacy benefits only. The imported source rows are not an
eligibility engine. They must not produce a positive match without evaluating
the joint conditions and exact olympiad year. Complete rules remain available
through `/admissions/rules` regardless of program mapping.

`admission_metadata` on programs preserves source affiliations and coverage.
`places_known`, `cost_known` and `subjects_known` are false: these quantities were
not collected. Unknown `budget_places`, `paid_places` and `cost` are stored as
SQL NULL and returned as JSON null. Confirmed zero values remain zero. Updated
iOS views display “Нет данных” in both lists and the initial/refreshed detail
card, while retaining compatibility with older zero values marked unknown. A level of zero on a catalogue olympiad
means its source rows do not establish a single RSOSH level, not “level 0”.

## Validation

`go -C api test ./...` checks lossless rule responses, null unknown minima,
grouping and multiple affiliations. Before the production import, the migration
and full generated SQL were applied twice to a temporary PostgreSQL clone:
7 universities, 379 programs, 7,891 rules, 20,196 program/rule associations.
The clone was removed afterwards. Public API checks should verify all seven
university program trees, nested program/olympiad benefits and paginated rules.

## Стоимость и места приёма 2026

Дополнение из `olympguide-static/data_loader/admissions/releases/2026-quantities.json`
проверяется по SHA-256 и накладывается на каталог по стабильным идентификаторам.
Полный импорт также использует его и не возвращает цены/места к пустым значениям.
Для уже загруженного каталога:

```console
python deploy/import_admissions.py --static-repo ../olympguide-static --quantities-only --host 158.160.150.215
python deploy/verify_quantities.py --static-repo ../olympguide-static
```

Импорт создаёт резервную копию, проверяет соответствие всех 379 программ вузам
и кодам направлений, обновляет три числовых поля и их происхождение в одной
транзакции. Числа общих конкурсных групп нельзя суммировать по профилям.
`admission_metadata.cost_period` — `year` или `semester`; у МИФИ опубликован
осенний семестр. Клиент обязан учитывать период и показывать `places_note`,
`quantity_notes` и ссылки `quantity_evidence.*.source_url`.

На 08.09.2026 подтверждены 372 бюджетных показателя, 365 платных и 367 тарифов.
Оставшиеся значения имеют `null` и пояснение; тариф на семестр не преобразуется
в годовой. Подробный охват и исключения — в `admissions/QUANTITIES.md` static-репозитория.
