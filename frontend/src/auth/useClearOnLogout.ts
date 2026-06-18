import { useEffect, useRef } from 'react'
import { useAuth } from './context'

// Runs `reset` whenever there's no token (i.e. on logout), so a component can
// wipe its last response. Reset is kept in a ref so callers don't have to
// memoize it; the effect only re-fires when the auth state changes.
export function useClearOnLogout(reset: () => void) {
  const { isAuthenticated } = useAuth()
  const resetRef = useRef(reset)
  resetRef.current = reset

  useEffect(() => {
    if (!isAuthenticated) {
      resetRef.current()
    }
  }, [isAuthenticated])
}
