import { useState } from 'react'
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
import { ApiError } from '../api'
import type { AccountCreatedResponse, AccountType } from '../types'
import { useAuth } from '../auth/context'
import { glowBorderSx } from '../animations'

const ACCOUNT_TYPES: AccountType[] = [
  'USER_CASH',
  'EXTERNAL',
  'CARD_SETTLEMENT',
  'ACH_CLEARING',
  'FEE_REVENUE',
  'TREASURY',
]

// Calls POST /api/signup via the auth context (signup returns a token, so it
// logs you in immediately). Glowing, like the login form.
export default function SignupPanel() {
  const { signup } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [accountType, setAccountType] = useState<AccountType>('USER_CASH')
  const [currency, setCurrency] = useState('USD')
  const [result, setResult] = useState<AccountCreatedResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function handleSubmit() {
    setError(null)
    setResult(null)
    setBusy(true)
    try {
      const res = await signup({
        email,
        password,
        account_type: accountType,
        currency,
      })
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
    <Paper elevation={0} sx={{ p: 3, width: '100%', ...glowBorderSx }}>
      <Stack spacing={2}>
        {/* Title */}
        <Box>
          <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
            <Chip label="POST" size="small" color="primary" />
            <Typography variant="subtitle1" sx={{ fontFamily: 'monospace' }}>
              /api/signup
            </Typography>
          </Stack>
          <Typography variant="caption" color="text.secondary">
            Create a user + first account, returns a token
          </Typography>
        </Box>

        {/* Inputs */}
        <TextField
          label="Email"
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          size="small"
          fullWidth
        />
        <TextField
          label="Password"
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          size="small"
          fullWidth
          helperText="Min 8 characters"
        />
        <TextField
          select
          label="Account type"
          value={accountType}
          onChange={(e) => setAccountType(e.target.value as AccountType)}
          size="small"
          fullWidth
        >
          {ACCOUNT_TYPES.map((t) => (
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

        {/* Response — between the inputs and the button */}
        {busy && (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 1 }}>
            <CircularProgress size={24} />
          </Box>
        )}
        {error && <Alert severity="error">{error}</Alert>}
        {result && !busy && (
          <Alert severity="success">
            Created {result.account_type} · {result.currency} — token issued
            <br />
            {result.account_id}
          </Alert>
        )}

        {/* Execute */}
        <Button variant="contained" onClick={handleSubmit} disabled={busy}>
          {busy ? 'Loading…' : 'Send request'}
        </Button>
      </Stack>
    </Paper>
  )
}
