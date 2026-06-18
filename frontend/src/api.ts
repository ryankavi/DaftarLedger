import type {
  AccountBalanceResponse,
  AccountCreatedResponse,
  CreateAccountRequest,
  GetAccountsResponse,
  LoginResponse,
  ReverseRequest,
  SignupRequest,
  TransactionRequest,
  TransactionResponse,
} from './types'

// All routes live under /api; the Vite dev proxy forwards this to the backend.
export const API_BASE = '/api'

// Module-level bearer token. AuthProvider keeps this in sync so every request
// is "auto-filled" without threading the token through each call site.
let authToken: string | null = null

export function setAuthToken(token: string | null): void {
  authToken = token
}

// Thrown on any non-2xx response. Carries the HTTP status so callers can branch
// (e.g. 401 -> log out, 403 -> hide an action).
export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers)
  headers.set('Accept', 'application/json')
  if (options.body !== undefined) {
    headers.set('Content-Type', 'application/json')
  }
  if (authToken) {
    headers.set('Authorization', `Bearer ${authToken}`)
  }

  const res = await fetch(`${API_BASE}${path}`, { ...options, headers })

  if (!res.ok) {
    // Error bodies are text/plain (http.Error), not JSON — read as text.
    const body = (await res.text()).trim()
    throw new ApiError(res.status, body || res.statusText)
  }

  // 204 No Content (account lifecycle ops) has no body to parse.
  if (res.status === 204) {
    return undefined as T
  }
  return (await res.json()) as T
}

// ---- Auth ----

export function login(email: string, password: string): Promise<LoginResponse> {
  return request<LoginResponse>('/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
}

export function signup(input: SignupRequest): Promise<AccountCreatedResponse> {
  return request<AccountCreatedResponse>('/signup', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

// ---- Accounts ----

export function listAccounts(): Promise<GetAccountsResponse> {
  return request<GetAccountsResponse>('/accounts')
}

export function createAccount(input: CreateAccountRequest): Promise<AccountCreatedResponse> {
  return request<AccountCreatedResponse>('/accounts', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function getAccountBalance(
  accountId: string,
  asOf?: string,
): Promise<AccountBalanceResponse> {
  const query = asOf ? `?as_of=${encodeURIComponent(asOf)}` : ''
  return request<AccountBalanceResponse>(`/accounts/${accountId}/balance${query}`)
}

export function closeAccount(accountId: string): Promise<void> {
  return request<void>(`/accounts/${accountId}/close`, { method: 'POST' })
}

export function freezeAccount(accountId: string): Promise<void> {
  return request<void>(`/accounts/${accountId}/freeze`, { method: 'POST' })
}

export function reopenAccount(accountId: string): Promise<void> {
  return request<void>(`/accounts/${accountId}/reopen`, { method: 'POST' })
}

// ---- Transactions ----
// The five forward flows share TransactionRequest and differ only by path.

function postTransaction(
  path: string,
  input: TransactionRequest,
): Promise<TransactionResponse> {
  return request<TransactionResponse>(path, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export const transfer = (input: TransactionRequest) =>
  postTransaction('/transactions/transfer', input)
export const deposit = (input: TransactionRequest) =>
  postTransaction('/transactions/deposit', input)
export const withdraw = (input: TransactionRequest) =>
  postTransaction('/transactions/withdraw', input)
export const assessFee = (input: TransactionRequest) =>
  postTransaction('/transactions/assess-fee', input)
export const refundFee = (input: TransactionRequest) =>
  postTransaction('/transactions/refund-fee', input)

export function reverseTransaction(
  transactionId: string,
  input: ReverseRequest,
): Promise<TransactionResponse> {
  return request<TransactionResponse>(`/transactions/${transactionId}/reverse`, {
    method: 'POST',
    body: JSON.stringify(input),
  })
}
