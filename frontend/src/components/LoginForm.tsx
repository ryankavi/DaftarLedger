import { useEffect, useState, type FormEvent } from 'react'
import Alert from '@mui/material/Alert'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import IconButton from '@mui/material/IconButton'
import InputAdornment from '@mui/material/InputAdornment'
import Paper from '@mui/material/Paper'
import Stack from '@mui/material/Stack'
import TextField from '@mui/material/TextField'
import EmailOutlinedIcon from '@mui/icons-material/EmailOutlined'
import LockOutlinedIcon from '@mui/icons-material/LockOutlined'
import Visibility from '@mui/icons-material/Visibility'
import VisibilityOff from '@mui/icons-material/VisibilityOff'
import { ApiError } from '../api'
import { useAuth } from '../auth/context'
import { glowBorderSx } from '../animations'
import PanelTitle from './PanelTitle'

// Minimal, borderless login: underlined `standard` inputs with leading icons
// and a show/hide password toggle. Self-contained — owns its form state and
// talks to the auth context directly, so App only has to render it.
export default function LoginForm() {
  const { login, logoutNonce, isAuthenticated } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  // Clear the form on logout (the grid no longer remounts to do it for us).
  useEffect(() => {
    setEmail('')
    setPassword('')
    setShowPassword(false)
    setError(null)
  }, [logoutNonce])

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setBusy(true)
    // Clear the inputs on submit. login() below still uses the values captured
    // in this render's closure, so the request keeps the entered credentials.
    setEmail('')
    setPassword('')
    try {
      await login(email, password)
    } catch (err) {
      setError(
        err instanceof ApiError ? `${err.status}: ${err.message}` : 'Login failed',
      )
    } finally {
      setBusy(false)
    }
  }

  return (
    <Box sx={{ width: '100%' }}>
      <Paper
        elevation={0}
        sx={{
          width: '100%',
          p: 4,
          // Glow only until a token is loaded; calm once authenticated.
          ...(isAuthenticated ? {} : glowBorderSx),
        }}
      >
        <Stack component="form" spacing={3} onSubmit={handleSubmit}>
          <Stack spacing={1} sx={{ alignItems: 'center' }}>
            <PanelTitle>Request Auth Token</PanelTitle>
          </Stack>
          {error && <Alert severity="error">{error}</Alert>}
          <TextField
            variant="standard"
            label="Email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
            autoFocus
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
            required
            fullWidth
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
          <Button type="submit" variant="contained" size="large" disabled={busy}>
            {busy ? 'Signing in…' : 'Log in'}
          </Button>
        </Stack>
      </Paper>
    </Box>
  )
}
