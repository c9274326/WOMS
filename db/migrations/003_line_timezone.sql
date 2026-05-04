ALTER TABLE production_lines
ADD COLUMN IF NOT EXISTS timezone TEXT NOT NULL DEFAULT 'Asia/Taipei';

UPDATE production_lines
SET timezone = 'Asia/Taipei'
WHERE timezone IS NULL OR timezone = '';

UPDATE production_lines
SET timezone = 'Europe/London'
WHERE id = 'D';
