import type { UserRole } from '../types'

// Reads the `role` claim from a JWT WITHOUT verifying its signature. Safe for UI
// hints only (e.g. showing a role badge, gating admin affordances) — the server
// re-verifies the signature and enforces authz on every request, so a tampered
// token gains nothing here.
export function decodeTokenRole(token: string): UserRole | null {
  try {
    const payload = token.split('.')[1]
    if (!payload) return null
    // JWT uses base64url; convert to base64 and re-pad before atob.
    const b64 = payload.replace(/-/g, '+').replace(/_/g, '/')
    const padded = b64 + '='.repeat((4 - (b64.length % 4)) % 4)
    const data = JSON.parse(atob(padded)) as { role?: unknown }
    return data.role === 'ADMIN' || data.role === 'USER' ? data.role : null
  } catch {
    return null
  }
}
