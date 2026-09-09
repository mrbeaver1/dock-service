ALTER TABLE documents ALTER COLUMN json_data TYPE json USING json_data::json;
