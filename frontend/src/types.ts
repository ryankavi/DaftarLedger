// DTOs mirroring openapi.yaml. Keep in sync with the spec / Go handlers.

export type AccountType =
  | 'USER_CASH'
  | 'EXTERNAL'
  | 'CARD_SETTLEMENT'
  | 'ACH_CLEARING'
  | 'FEE_REVENUE'
  | 'TREASURY'

export type AccountStatus = 'OPEN' | 'FROZEN' | 'CLOSED'

export type TransactionStatus = 'PENDING' | 'POSTED' | 'REVERSED'

export type UserRole = 'USER' | 'ADMIN'

export interface LoginResponse {
  token: string
}

export interface SignupRequest {
  email: string
  password: string
  account_type: AccountType
  currency: string
}

// signup and createAccount share this response shape; createAccount returns an
// empty token (you already hold one).
export interface AccountCreatedResponse {
  account_id: string
  account_type: AccountType
  currency: string
  token: string
}

export interface CreateAccountRequest {
  account_type: AccountType
  currency: string
}

export interface Account {
  account_id: string
  account_type: AccountType
  account_status: AccountStatus
  currency: string
}

export interface GetAccountsResponse {
  accounts: Account[]
}

export interface AccountBalanceResponse {
  balance: number
}

export interface TransactionRequest {
  from_account_id: string
  to_account_id: string
  external_id?: string | null
  amount: number
  currency: string
  idempotency_key: string
  memo?: string | null
  description?: string | null
}

export interface TransactionResponse {
  transaction_id: string
  external_id?: string | null
  idempotency_key: string
  transaction_type: string
  description?: string | null
  transaction_status: TransactionStatus
  posted_at?: string | null
  created_at: string
}

export interface ReverseRequest {
  idempotency_key: string
  memo?: string | null
}
