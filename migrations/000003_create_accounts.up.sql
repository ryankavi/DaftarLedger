DO $$ BEGIN
    CREATE TYPE account_type AS ENUM (
        'USER_CASH',
        'CARD_SETTLEMENT',
        'ACH_CLEARING',
        'FEE_REVENUE',
        'SYSTEM'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

CREATE TABLE IF NOT EXISTS accounts (
    account_id UUID PRIMARY KEY,
    account_type account_type NOT NULL,
    owner_id UUID REFERENCES users(user_id) NOT NULL,
    currency CHAR(3),
    created_at TIMESTAMPTZ DEFAULT NOW()
);
