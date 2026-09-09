-- Membatalkan 000027.
ALTER TABLE tenants DROP CONSTRAINT IF EXISTS fk_tenants_referred_by_partner;
-- Program mitra dilepas → tautan tenant→mitra tak lagi punya makna referensial.
UPDATE tenants SET referred_by_partner_id = NULL WHERE referred_by_partner_id IS NOT NULL;
DROP TABLE IF EXISTS partner_referrals;
DROP TABLE IF EXISTS partner_leads;
