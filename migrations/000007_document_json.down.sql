LOCK TABLE documents IN ACCESS EXCLUSIVE MODE;

UPDATE documents SET json_data = json_data::jsonb::json
WHERE json_data::text IS DISTINCT FROM json_data::jsonb::text;

ALTER TABLE documents ALTER COLUMN json_data TYPE jsonb USING json_data::jsonb;
