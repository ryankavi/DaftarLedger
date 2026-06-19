-- Seed the platform accounts (and a system user to own them) with fixed UUIDs,
-- so they exist automatically on every deploy and their ids are stable/known.
-- Idempotent: ON CONFLICT DO NOTHING makes re-runs (or a partially-applied
-- state) safe.

-- A deterministic "system" user owns the platform accounts (accounts.owner_id
-- is NOT NULL). It can never log in: 'x' is not a valid bcrypt hash, so
-- CompareHashAndPassword always fails.
INSERT INTO users (user_id, email, password_hash, role)
VALUES ('00000000-0000-0000-0000-000000000001', 'system@payclone.local', 'x', 'ADMIN')
ON CONFLICT DO NOTHING;

-- Fixed ids so the app/docs can reference them directly (e.g. the deposit
-- "from" field) without a lookup. One per type (UNIQUE(owner_id, account_type)).
INSERT INTO accounts (account_id, account_type, owner_id, currency) VALUES
  ('a0000000-0000-0000-0000-000000000001', 'EXTERNAL',        '00000000-0000-0000-0000-000000000001', 'USD'),
  ('a0000000-0000-0000-0000-000000000002', 'TREASURY',        '00000000-0000-0000-0000-000000000001', 'USD'),
  ('a0000000-0000-0000-0000-000000000003', 'FEE_REVENUE',     '00000000-0000-0000-0000-000000000001', 'USD'),
  ('a0000000-0000-0000-0000-000000000004', 'CARD_SETTLEMENT', '00000000-0000-0000-0000-000000000001', 'USD'),
  ('a0000000-0000-0000-0000-000000000005', 'ACH_CLEARING',    '00000000-0000-0000-0000-000000000001', 'USD')
ON CONFLICT DO NOTHING;
