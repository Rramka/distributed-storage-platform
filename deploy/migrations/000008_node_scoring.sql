-- M4 follow-up: scheduler scoring inputs (docs/06-scheduler-and-repair.md).

ALTER TABLE nodes ADD COLUMN IF NOT EXISTS probation_until TIMESTAMPTZ;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS reputation_updated_at TIMESTAMPTZ;

UPDATE nodes
SET probation_until = registered_at + interval '14 days'
WHERE probation_until IS NULL;
