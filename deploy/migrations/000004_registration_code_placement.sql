-- M3: placement attributes on registration codes (docs/06-scheduler-and-repair.md, docs/08-api.md).
-- Provider declares country/region/asn when minting; copied onto nodes at register.

ALTER TABLE node_registration_codes ADD COLUMN IF NOT EXISTS country TEXT;
ALTER TABLE node_registration_codes ADD COLUMN IF NOT EXISTS region TEXT;
ALTER TABLE node_registration_codes ADD COLUMN IF NOT EXISTS asn INTEGER;
