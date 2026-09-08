--liquibase formatted sql
CREATE TABLE olympguide.scholarship (
    id TEXT PRIMARY KEY,
    university_key TEXT NOT NULL REFERENCES olympguide.university(catalog_key),
    payload JSONB NOT NULL CHECK (jsonb_typeof(payload) = 'object')
);
CREATE INDEX scholarship_university ON olympguide.scholarship(university_key);
