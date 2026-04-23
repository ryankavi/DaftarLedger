DO $$ BEGIN
    CREATE TYPE transaction_status AS ENUM (
        'PENDING',
        'POSTED',
        'REVERSED'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

CREATE TABLE IF NOT EXISTS transactions (
    transaction_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id UUID,
    idempotency_key TEXT UNIQUE NOT NULL,
    transaction_type TEXT NOT NULL,
    transaction_description TEXT,
    transaction_status transaction_status NOT NULL,
    posted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
