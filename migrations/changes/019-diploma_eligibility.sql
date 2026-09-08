--liquibase formatted sql
ALTER TABLE olympguide.diploma ADD award_year INTEGER CHECK(award_year BETWEEN 2000 AND 2100);
ALTER TABLE olympguide.diploma ADD olympiad_year TEXT;
ALTER TABLE olympguide.diploma ADD profile TEXT;
ALTER TABLE olympguide.diploma ADD result TEXT CHECK(result IN ('winner','prize_winner','participant'));
ALTER TABLE olympguide.diploma DROP CONSTRAINT diploma_class_checker;
ALTER TABLE olympguide.diploma ADD CONSTRAINT diploma_class_checker CHECK(class BETWEEN 1 AND 11);
ALTER TABLE olympguide.diploma DROP CONSTRAINT diploma_user_id_olympiad_id_class_key;
CREATE UNIQUE INDEX diploma_identity_year ON olympguide.diploma(user_id,olympiad_id,class,COALESCE(award_year,0),COALESCE(profile,''));
CREATE TABLE olympguide.user_admission_profile (
    user_id INTEGER PRIMARY KEY REFERENCES olympguide."user" ON DELETE CASCADE,
    payload JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE olympguide.admission_evaluation (
    id TEXT PRIMARY KEY,
    admission_year INTEGER NOT NULL,
    rule_id TEXT NOT NULL,
    payload JSONB NOT NULL,
    source_payload JSONB NOT NULL,
    FOREIGN KEY(admission_year,rule_id) REFERENCES olympguide.admission_rule ON DELETE CASCADE
);
CREATE INDEX admission_evaluation_rule ON olympguide.admission_evaluation(admission_year,rule_id);
