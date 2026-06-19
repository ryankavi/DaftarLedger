-- Remove the seeded platform accounts, then the system user (FK order).
DELETE FROM accounts WHERE owner_id = '00000000-0000-0000-0000-000000000001';
DELETE FROM users    WHERE user_id  = '00000000-0000-0000-0000-000000000001';
