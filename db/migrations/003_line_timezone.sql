ALTER TABLE production_lines
ADD COLUMN IF NOT EXISTS timezone TEXT NOT NULL DEFAULT 'Asia/Taipei';

UPDATE production_lines
SET timezone = 'Asia/Taipei'
WHERE timezone IS NULL OR timezone = '';
