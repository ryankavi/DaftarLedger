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

DO $$ BEGIN
    CREATE TYPE account_status AS ENUM (
        'OPEN',
        'FROZEN',
        'CLOSED'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

CREATE TABLE IF NOT EXISTS accounts (
    account_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_type account_type NOT NULL,
    account_status account_status NOT NULL DEFAULT 'OPEN',
    owner_id UUID NOT NULL REFERENCES users(user_id),
    currency VARCHAR(3) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (owner_id, account_type)
);
