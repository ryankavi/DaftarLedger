import { useEffect, useState } from 'react'
import Alert from '@mui/material/Alert'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Chip from '@mui/material/Chip'
import CircularProgress from '@mui/material/CircularProgress'
import MenuItem from '@mui/material/MenuItem'
import Paper from '@mui/material/Paper'
import Stack from '@mui/material/Stack'
import TextField from '@mui/material/TextField'
import Typography from '@mui/material/Typography'
import { ApiError, createAccount } from '../api'
import { useAuth } from '../auth/context'
import type { AccountCreatedResponse, AccountType, UserRole } from '../types'

// Account creation always owns the new account to the caller, so the two roles
// create disjoint sets: a USER self-serves their own wallet; an ADMIN provisions
// the platform accounts (an admin has no business owning a USER_CASH wallet).
const ADMIN_ACCOUNT_TYPES: AccountType[] = [
  'EXTERNAL',
  'CARD_SETTLEMENT',
  'ACH_CLEARING',
  'FEE_REVENUE',
  'TREASURY',
]

// USER_CASH is the only self-serve type; the platform types above are gated to
// admins by the backend (a non-admin POST gets a 403).
const USER_ACCOUNT_TYPES: AccountType[] = ['USER_CASH']

// The pre-selected type for each role — the first entry of their allowed list.
function defaultAccountType(role: UserRole | null): AccountType {
  return role === 'ADMIN' ? ADMIN_ACCOUNT_TYPES[0] : USER_ACCOUNT_TYPES[0]
}

// Calls POST /api/accounts to create an account for the current user. Wrapped
// in the same breathing glow as the login form.
export default function CreateAccountPanel() {
  const { isAuthenticated, role } = useAuth()
  // Squish shut only when there's no token at all; any authenticated caller
  // (USER or ADMIN) sees the form.
  const expanded = isAuthenticated
  // Admins can provision platform accounts; everyone else only USER_CASH.
  const allowedTypes = role === 'ADMIN' ? ADMIN_ACCOUNT_TYPES : USER_ACCOUNT_TYPES

  const [accountType, setAccountType] = useState<AccountType>(() =>
    defaultAccountType(role),
  )
  const [currency, setCurrency] = useState('USD')
  const [result, setResult] = useState<AccountCreatedResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  // Reset on login (not logout): we keep the last response on screen through
  // logout so the panel can animate closed with content to squish, then clear
  // any leftover state when a fresh token arrives. The default type follows the
  // role, so an admin lands on a platform type rather than an absent USER_CASH.
  useEffect(() => {
    if (!isAuthenticated) return
    setAccountType(defaultAccountType(role))
    setCurrency('USD')
    setResult(null)
    setError(null)
  }, [isAuthenticated, role])

  // Keep the current selection in range even while the role is narrowing it
  // (e.g. mid-logout), so MUI doesn't warn about an out-of-range Select value.
  const accountTypes = allowedTypes.includes(accountType)
    ? allowedTypes
    : [accountType, ...allowedTypes]

  async function handleSubmit() {
    setError(null)
    setResult(null)
    setBusy(true)
    try {
      const res = await createAccount({ account_type: accountType, currency })
      setResult(res)
    } catch (err) {
      setError(
        err instanceof ApiError ? `${err.status}: ${err.message}` : 'Request failed',
      )
    } finally {
      setBusy(false)
    }
  }

  return (
    <Paper sx={{ p: 3, width: '100%' }}>
      {/* Title — always visible, even when the body is squished shut */}
      <Box>
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
          <Chip label="POST" size="small" color="primary" />
          <Typography variant="subtitle1" sx={{ fontFamily: 'monospace' }}>
            /api/accounts
          </Typography>
        </Stack>
        <Typography variant="caption" color="text.secondary">
          Create an account for the current user
        </Typography>
      </Box>

      {/* Body — collapses with a vertical squish (top down, bottom up) when the
          caller has no token, or is authenticated as a non-USER role. */}
      <Box
        aria-hidden={!expanded}
        sx={{
          overflow: 'hidden',
          transformOrigin: 'center',
          transition: (theme) =>
            theme.transitions.create(['max-height', 'transform', 'opacity'], {
              duration: theme.transitions.duration.standard,
            }),
          maxHeight: expanded ? '40rem' : 0,
          transform: expanded ? 'scaleY(1)' : 'scaleY(0)',
          opacity: expanded ? 1 : 0,
          pointerEvents: expanded ? 'auto' : 'none',
        }}
      >
        <Stack spacing={2} sx={{ mt: 2 }}>
          {/* Inputs */}
          <TextField
            select
            label="Account type"
            value={accountType}
            onChange={(e) => setAccountType(e.target.value as AccountType)}
            size="small"
            fullWidth
          >
            {accountTypes.map((t) => (
              <MenuItem key={t} value={t}>
                {t}
              </MenuItem>
            ))}
          </TextField>
          <TextField
            label="Currency"
            value={currency}
            onChange={(e) => setCurrency(e.target.value.toUpperCase())}
            size="small"
            fullWidth
            slotProps={{ htmlInput: { maxLength: 3 } }}
          />

          {/* Response — between the title/inputs and the button */}
          {busy && (
            <Box sx={{ display: 'flex', justifyContent: 'center', py: 1 }}>
              <CircularProgress size={24} />
            </Box>
          )}
          {error && <Alert severity="error">{error}</Alert>}
          {result && !busy && (
            <Alert severity="success">
              Created {result.account_type} · {result.currency}
              <br />
              {result.account_id}
            </Alert>
          )}

          {/* Execute */}
          <Button variant="contained" onClick={handleSubmit} disabled={busy}>
            {busy ? 'Loading…' : 'Send request'}
          </Button>
        </Stack>
      </Box>
    </Paper>
  )
}
