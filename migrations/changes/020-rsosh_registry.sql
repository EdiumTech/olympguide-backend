--liquibase formatted sql
-- Academic seasons are separate from admission years. Keep historical diploma IDs.
ALTER TABLE olympguide.olympiad ADD academic_year TEXT;
ALTER TABLE olympguide.olympiad ADD registry_status TEXT CHECK (registry_status IN ('draft','approved'));
ALTER TABLE olympguide.olympiad ADD registry_active BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE olympguide.olympiad ADD subjects TEXT;
ALTER TABLE olympguide.olympiad ADD CONSTRAINT registry_entry_valid CHECK (
    academic_year IS NULL OR (
      academic_year ~ '^[0-9]{4}/[0-9]{4}$' AND category IS NOT NULL AND category='rsosh'
      AND level BETWEEN 1 AND 3 AND length(trim(profile))>0 AND subjects IS NOT NULL
      AND length(trim(subjects))>0 AND registry_status IS NOT NULL AND admission_year IS NULL
    )
);
CREATE INDEX olympiad_registry_season ON olympguide.olympiad(academic_year) WHERE registry_active;
CREATE TABLE olympguide.olympiad_registry_release (
    academic_year TEXT PRIMARY KEY,
    status TEXT NOT NULL CHECK (status IN ('draft','approved')),
    payload JSONB NOT NULL,
    imported_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE olympguide.university ADD contact_metadata JSONB;
