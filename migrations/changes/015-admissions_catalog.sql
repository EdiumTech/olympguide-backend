--liquibase formatted sql
-- Stable catalogue keys preserve API IDs and favourites across repeated imports.
ALTER TABLE olympguide.university ADD catalog_key TEXT UNIQUE;
ALTER TABLE olympguide.faculty ADD catalog_key TEXT UNIQUE;
ALTER TABLE olympguide.faculty ADD parent_id INTEGER REFERENCES olympguide.faculty(faculty_id);
ALTER TABLE olympguide.faculty ADD unit_type TEXT;
ALTER TABLE olympguide.educational_program ADD catalog_key TEXT UNIQUE;
ALTER TABLE olympguide.educational_program ADD admission_metadata JSONB;
ALTER TABLE olympguide.educational_program ALTER faculty_id DROP NOT NULL;
ALTER TABLE olympguide.educational_program DROP CONSTRAINT name_program_unique;
ALTER TABLE olympguide.olympiad ADD catalog_key TEXT UNIQUE;
ALTER TABLE olympguide.olympiad ADD admission_year INTEGER;
ALTER TABLE olympguide.olympiad ADD category TEXT;
-- Zero means that a single RSOSH level cannot describe all source rows.
ALTER TABLE olympguide.olympiad DROP CONSTRAINT olympiad_level_checker;
ALTER TABLE olympguide.olympiad ADD CONSTRAINT olympiad_level_checker CHECK (level BETWEEN 0 AND 3);

-- Seed migrations used explicit IDs without advancing SERIAL sequences.
SELECT setval('olympguide.olympiad_olympiad_id_seq', GREATEST(
    (SELECT last_value FROM olympguide.olympiad_olympiad_id_seq),
    COALESCE((SELECT max(olympiad_id) FROM olympguide.olympiad), 1)), true);
SELECT setval('olympguide.field_of_study_field_id_seq', GREATEST(
    (SELECT last_value FROM olympguide.field_of_study_field_id_seq),
    COALESCE((SELECT max(field_id) FROM olympguide.field_of_study), 1)), true);
SELECT setval('olympguide.group_of_fields_group_id_seq', GREATEST(
    (SELECT last_value FROM olympguide.group_of_fields_group_id_seq),
    COALESCE((SELECT max(group_id) FROM olympguide.group_of_fields), 1)), true);

CREATE TABLE olympguide.program_faculty (
    program_id INTEGER REFERENCES olympguide.educational_program ON DELETE CASCADE,
    faculty_id INTEGER REFERENCES olympguide.faculty ON DELETE CASCADE,
    PRIMARY KEY(program_id, faculty_id)
);
CREATE TABLE olympguide.admission_release (
    admission_year INTEGER PRIMARY KEY,
    manifest_sha256 TEXT NOT NULL,
    payload JSONB NOT NULL,
    imported_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE olympguide.admission_source (
    admission_year INTEGER REFERENCES olympguide.admission_release,
    id TEXT NOT NULL,
    payload JSONB NOT NULL,
    PRIMARY KEY(admission_year, id)
);
CREATE TABLE olympguide.admission_rule (
    admission_year INTEGER REFERENCES olympguide.admission_release,
    id TEXT NOT NULL,
    university_id INTEGER NOT NULL REFERENCES olympguide.university,
    olympiad_id INTEGER NOT NULL REFERENCES olympguide.olympiad,
    source_id TEXT NOT NULL,
    category TEXT NOT NULL,
    payload JSONB NOT NULL,
    PRIMARY KEY(admission_year, id),
    FOREIGN KEY(admission_year, source_id) REFERENCES olympguide.admission_source
);
CREATE INDEX admission_rule_university ON olympguide.admission_rule(university_id, admission_year);
CREATE INDEX admission_rule_olympiad ON olympguide.admission_rule(olympiad_id, admission_year);
CREATE TABLE olympguide.admission_program_rule (
    admission_year INTEGER NOT NULL,
    rule_id TEXT NOT NULL,
    program_id INTEGER REFERENCES olympguide.educational_program,
    relation TEXT NOT NULL CHECK (relation IN ('program_conditions', 'field_conditions', 'school_conditions')),
    PRIMARY KEY(admission_year, rule_id, program_id),
    FOREIGN KEY(admission_year, rule_id) REFERENCES olympguide.admission_rule ON DELETE CASCADE
);
CREATE INDEX admission_program_rule_program ON olympguide.admission_program_rule(program_id);
