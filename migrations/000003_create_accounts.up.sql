DO $$ BEGIN
    CREATE TYPE account_type AS ENUM (
        'USER_CASH',
        'EXTERNAL',
        'CARD_SETTLEMENT',
        'ACH_CLEARING',
        'FEE_REVENUE',
        'TREASURY'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

CREATE TABLE IF NOT EXISTS accounts (
    account_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_type account_type NOT NULL,
    owner_id UUID NOT NULL REFERENCES users(user_id),
    currency CHAR(3) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
