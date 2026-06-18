import { useCallback, useState, type ReactNode } from 'react'
import { login as loginRequest, signup as signupRequest, setAuthToken } from '../api'
import type { SignupRequest } from '../types'
import { AuthContext } from './context'
import { decodeTokenRole } from './jwt'

// Demo-grade token storage: in sessionStorage so a refresh survives but the
// token is gone when the tab closes. No refresh token, no revocation — the
// backend's short TTL is the safety net (see PLAN.md). "Logout" just clears it.
const TOKEN_KEY = 'payclone.token'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() =>
    sessionStorage.getItem(TOKEN_KEY),
  )
  // Bumped on every logout() so views can fully reset even when no token was
  // loaded (logging out twice still triggers a change).
  const [logoutNonce, setLogoutNonce] = useState(0)

  // Sync the api client's bearer header during render (not in an effect): child
  // data-fetching effects run before parent effects, so an effect here could
  // leave the first post-refresh request unauthenticated. This is idempotent.
  setAuthToken(token)

  // Read the role straight from the token's claim — no API call. UI hint only.
  const role = token ? decodeTokenRole(token) : null

  const login = useCallback(async (email: string, password: string) => {
    const res = await loginRequest(email, password)
    sessionStorage.setItem(TOKEN_KEY, res.token)
    setToken(res.token)
  }, [])

  // Signup also returns a token, so it authenticates immediately — same path as
  // login. Returns the created-account response so the caller can display it.
  const signup = useCallback(async (input: SignupRequest) => {
    const res = await signupRequest(input)
    sessionStorage.setItem(TOKEN_KEY, res.token)
    setToken(res.token)
    return res
  }, [])

  const logout = useCallback(() => {
    sessionStorage.removeItem(TOKEN_KEY)
    setToken(null)
    setLogoutNonce((n) => n + 1)
  }, [])

  return (
    <AuthContext.Provider
      value={{
        token,
        isAuthenticated: token !== null,
        role,
        logoutNonce,
        login,
        signup,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}
