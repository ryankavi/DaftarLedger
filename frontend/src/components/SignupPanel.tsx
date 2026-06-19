import { useEffect, useState } from 'react'
import Alert from '@mui/material/Alert'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Chip from '@mui/material/Chip'
import CircularProgress from '@mui/material/CircularProgress'
import IconButton from '@mui/material/IconButton'
import InputAdornment from '@mui/material/InputAdornment'
import Paper from '@mui/material/Paper'
import Stack from '@mui/material/Stack'
import TextField from '@mui/material/TextField'
import Typography from '@mui/material/Typography'
import Visibility from '@mui/icons-material/Visibility'
import VisibilityOff from '@mui/icons-material/VisibilityOff'
import EmailOutlinedIcon from '@mui/icons-material/EmailOutlined'
import LockOutlinedIcon from '@mui/icons-material/LockOutlined'
import AttachMoneyIcon from '@mui/icons-material/AttachMoney'
import { ApiError } from '../api'
import type { AccountCreatedResponse } from '../types'
import { useAuth } from '../auth/context'
import { glowBorderSx } from '../animations'
import PanelTitle from './PanelTitle'

// Calls POST /api/signup via the auth context (signup returns a token, so it
// logs you in immediately). Glowing, like the login form.
export default function SignupPanel() {
  const { signup, logoutNonce, isAuthenticated } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [currency, setCurrency] = useState('USD')
  const [result, setResult] = useState<AccountCreatedResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  // Reset on logout (not login): signup itself logs you in and then shows its
  // success result, so we keep that result until the token is cleared. The grid
  // no longer remounts to clear this for us.
  useEffect(() => {
    setEmail('')
    setPassword('')
    setShowPassword(false)
    setCurrency('USD')
    setResult(null)
    setError(null)
  }, [logoutNonce])

  async function handleSubmit() {
    setError(null)
    setResult(null)
    setBusy(true)
    try {
      const res = await signup({
        email,
        password,
        // Signup only ever opens a USER_CASH wallet; platform accounts are
        // admin-provisioned, so there's nothing to choose here.
        account_type: 'USER_CASH',
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
    <Paper
      elevation={0}
      sx={{
        p: 3,
        width: '100%',
        // Glow only until a token is loaded; calm once authenticated.
        ...(isAuthenticated ? {} : glowBorderSx),
      }}
    >
      <Stack spacing={2}>
        {/* Title */}
        <Box>
          <Box sx={{ textAlign: 'center' }}>
            <PanelTitle>Sign Up</PanelTitle>
          </Box>
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
          variant="standard"
          label="Email"
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          fullWidth
          slotProps={{
            input: {
              startAdornment: (
                <InputAdornment position="start">
                  <EmailOutlinedIcon fontSize="small" />
                </InputAdornment>
              ),
            },
          }}
        />
        <TextField
          variant="standard"
          label="Password"
          type={showPassword ? 'text' : 'password'}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          fullWidth
          helperText="Min 8 characters"
          slotProps={{
            input: {
              startAdornment: (
                <InputAdornment position="start">
                  <LockOutlinedIcon fontSize="small" />
                </InputAdornment>
              ),
              endAdornment: (
                <InputAdornment position="end">
                  <IconButton
                    onClick={() => setShowPassword((s) => !s)}
                    edge="end"
                    size="small"
                    aria-label={showPassword ? 'Hide password' : 'Show password'}
                  >
                    {showPassword ? (
                      <VisibilityOff fontSize="small" />
                    ) : (
                      <Visibility fontSize="small" />
                    )}
                  </IconButton>
                </InputAdornment>
              ),
            },
          }}
        />
        <TextField
          variant="standard"
          label="Currency"
          value={currency}
          onChange={(e) => setCurrency(e.target.value.toUpperCase())}
          fullWidth
          slotProps={{
            input: {
              startAdornment: (
                <InputAdornment position="start">
                  <AttachMoneyIcon fontSize="small" />
                </InputAdornment>
              ),
            },
            htmlInput: { maxLength: 3 },
          }}
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
