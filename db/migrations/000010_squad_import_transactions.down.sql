ALTER TABLE squad_import_events DROP CONSTRAINT IF EXISTS squad_import_event_target_check;
DELETE FROM squad_import_events WHERE mode='draft';
ALTER TABLE squad_import_events DROP COLUMN IF EXISTS draft_id;
ALTER TABLE squad_import_events ALTER COLUMN plan_id SET NOT NULL;
DROP TABLE IF EXISTS squad_import_drafts;
