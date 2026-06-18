import { createContext, useContext } from 'react'
import type { AccountCreatedResponse, SignupRequest, UserRole } from '../types'

export interface AuthContextValue {
  token: string | null
  isAuthenticated: boolean
  // Role decoded from the token's claim (not verified) — UI hint only.
  role: UserRole | null
  // Incremented on every logout() call (even with no token loaded). Views can
  // key off it to fully reset.
  logoutNonce: number
  login: (email: string, password: string) => Promise<void>
  signup: (input: SignupRequest) => Promise<AccountCreatedResponse>
  logout: () => void
}

export const AuthContext = createContext<AuthContextValue | null>(null)

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error('useAuth must be used within <AuthProvider>')
  }
  return ctx
}
