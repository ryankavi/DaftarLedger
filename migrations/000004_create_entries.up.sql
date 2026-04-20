DO $$ BEGIN
    CREATE TYPE entry_direction AS ENUM ('DEBIT', 'CREDIT');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

CREATE TABLE IF NOT EXISTS entries (
    entry_id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL,
    account_id     UUID NOT NULL REFERENCES accounts(account_id),
    amount         BIGINT NOT NULL,
    currency       VARCHAR(3) NOT NULL,
    direction      entry_direction NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_at   TIMESTAMPTZ NOT NULL,
    memo           TEXT
);
