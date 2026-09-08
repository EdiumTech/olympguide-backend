--liquibase formatted sql
-- An uncollected quantity is unknown, not a confirmed zero.
ALTER TABLE olympguide.educational_program ALTER budget_places DROP NOT NULL;
ALTER TABLE olympguide.educational_program ALTER paid_places DROP NOT NULL;
ALTER TABLE olympguide.educational_program ALTER cost DROP NOT NULL;

UPDATE olympguide.educational_program
SET budget_places = NULL, paid_places = NULL
WHERE admission_metadata->>'places_known' = 'false';

UPDATE olympguide.educational_program
SET cost = NULL
WHERE admission_metadata->>'cost_known' = 'false';
