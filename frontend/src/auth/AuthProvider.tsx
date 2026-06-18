import { useCallback, useState, type ReactNode } from 'react'
import { login as loginRequest, setAuthToken } from '../api'
import { AuthContext } from './context'

// Demo-grade token storage: in sessionStorage so a refresh survives but the
// token is gone when the tab closes. No refresh token, no revocation — the
// backend's short TTL is the safety net (see PLAN.md). "Logout" just clears it.
const TOKEN_KEY = 'payclone.token'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() =>
    sessionStorage.getItem(TOKEN_KEY),
  )

  // Sync the api client's bearer header during render (not in an effect): child
  // data-fetching effects run before parent effects, so an effect here could
  // leave the first post-refresh request unauthenticated. This is idempotent.
  setAuthToken(token)

  const login = useCallback(async (email: string, password: string) => {
    const res = await loginRequest(email, password)
    sessionStorage.setItem(TOKEN_KEY, res.token)
    setToken(res.token)
  }, [])

  const logout = useCallback(() => {
    sessionStorage.removeItem(TOKEN_KEY)
    setToken(null)
  }, [])

  return (
    <AuthContext.Provider
      value={{ token, isAuthenticated: token !== null, login, logout }}
    >
      {children}
    </AuthContext.Provider>
  )
}
