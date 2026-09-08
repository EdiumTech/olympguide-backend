--liquibase formatted sql
CREATE TABLE olympguide.calendar_event (
    id TEXT PRIMARY KEY,
    season TEXT NOT NULL,
    collection_key TEXT NOT NULL,
    revision TEXT NOT NULL,
    payload JSONB NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE olympguide.calendar_plan (
    plan_id BIGSERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES olympguide."user" ON DELETE CASCADE,
    event_id TEXT REFERENCES olympguide.calendar_event,
    client_key UUID,
    custom_event JSONB,
    status TEXT NOT NULL DEFAULT 'planned' CHECK(status IN ('planned','registered','participating','completed','skipped')),
    note TEXT NOT NULL DEFAULT '' CHECK(length(note)<=4000),
    seen_revision TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id,event_id),
    UNIQUE(user_id,client_key),
    CHECK((event_id IS NOT NULL AND custom_event IS NULL AND client_key IS NULL) OR
          (event_id IS NULL AND custom_event IS NOT NULL AND client_key IS NOT NULL))
);
CREATE INDEX calendar_plan_owner ON olympguide.calendar_plan(user_id);
CREATE INDEX calendar_event_season ON olympguide.calendar_event(season);
