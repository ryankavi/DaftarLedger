import { useState, type FormEvent } from 'react'
import Alert from '@mui/material/Alert'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Card from '@mui/material/Card'
import CardContent from '@mui/material/CardContent'
import Stack from '@mui/material/Stack'
import TextField from '@mui/material/TextField'
import Typography from '@mui/material/Typography'
import { ApiError } from './api'
import { useAuth } from './auth/context'

// Placeholder shell: a minimal login/logout flow that proves the foundation
// works end to end (MUI renders, AuthProvider holds the token, the Vite proxy
// reaches the backend). Real screens (accounts, transactions) replace this next.
export default function App() {
  const { isAuthenticated, token, login, logout } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function handleLogin(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setBusy(true)
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

  if (isAuthenticated) {
    return (
      <Box sx={{ minHeight: '100%', display: 'grid', placeItems: 'center', p: 2 }}>
        <Card sx={{ maxWidth: 520, width: '100%' }}>
          <CardContent>
            <Stack spacing={2}>
              <Typography variant="h6">Signed in</Typography>
              <Typography
                variant="body2"
                color="text.secondary"
                sx={{ wordBreak: 'break-all' }}
              >
                Token: {token}
              </Typography>
              <Button variant="outlined" onClick={logout}>
                Log out
              </Button>
            </Stack>
          </CardContent>
        </Card>
      </Box>
    )
  }

  return (
    <Box sx={{ minHeight: '100%', display: 'grid', placeItems: 'center', p: 2 }}>
      <Card sx={{ maxWidth: 400, width: '100%' }}>
        <CardContent>
          <Stack component="form" spacing={2} onSubmit={handleLogin}>
            <Typography variant="h5">PayClone</Typography>
            {error && <Alert severity="error">{error}</Alert>}
            <TextField
              label="Email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoFocus
            />
            <TextField
              label="Password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
            <Button type="submit" variant="contained" disabled={busy}>
              {busy ? 'Signing in…' : 'Log in'}
            </Button>
          </Stack>
        </CardContent>
      </Card>
    </Box>
  )
}
