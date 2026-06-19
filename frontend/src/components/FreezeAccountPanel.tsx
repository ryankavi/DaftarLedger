import { useEffect, useState } from 'react'
import Alert from '@mui/material/Alert'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Chip from '@mui/material/Chip'
import CircularProgress from '@mui/material/CircularProgress'
import InputAdornment from '@mui/material/InputAdornment'
import Paper from '@mui/material/Paper'
import Stack from '@mui/material/Stack'
import TextField from '@mui/material/TextField'
import Typography from '@mui/material/Typography'
import AccountBalanceWalletOutlinedIcon from '@mui/icons-material/AccountBalanceWalletOutlined'
import { ApiError, freezeAccount } from '../api'
import { useAuth } from '../auth/context'
import LockedFilm from './LockedFilm'
import PanelTitle from './PanelTitle'
import HelpPopover from './HelpPopover'

// Calls POST /api/accounts/{id}/freeze. Any authenticated user may freeze their
// own account, so it's user-gated (lock film).
export default function FreezeAccountPanel() {
  const { isAuthenticated } = useAuth()
  const expanded = isAuthenticated

  const [accountId, setAccountId] = useState('')
  const [done, setDone] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  // Reset on login (not logout): keep the last result on screen through logout
  // so the panel animates closed, then clear when a fresh token arrives.
  useEffect(() => {
    if (!isAuthenticated) return
    setAccountId('')
    setDone(false)
    setError(null)
  }, [isAuthenticated])

  async function handleSubmit() {
    setError(null)
    setDone(false)
    setBusy(true)
    try {
      await freezeAccount(accountId)
      setDone(true)
    } catch (err) {
      setError(
        err instanceof ApiError ? `${err.status}: ${err.message}` : 'Request failed',
      )
    } finally {
      setBusy(false)
    }
  }

  return (
    <Paper sx={{ p: 3, width: '100%', position: 'relative' }}>
      <HelpPopover>
        <p>
          Freeze an open account, blocking further activity until it's reopened.
        </p>
      </HelpPopover>

      {/* Title — always visible, even when the body is squished shut */}
      <Box>
        <PanelTitle>Freeze Account</PanelTitle>
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
          <Chip label="POST" size="small" color="primary" />
          <Typography variant="subtitle1" sx={{ fontFamily: 'monospace' }}>
            {'/api/accounts/{id}/freeze'}
          </Typography>
        </Stack>
        <Typography variant="caption" color="text.secondary">
          Freeze an open account
        </Typography>
      </Box>

      {/* Body — collapses with a vertical squish when there's no token. */}
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
            variant="standard"
            label="Account ID"
            value={accountId}
            onChange={(e) => setAccountId(e.target.value)}
            fullWidth
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <AccountBalanceWalletOutlinedIcon fontSize="small" />
                  </InputAdornment>
                ),
              },
            }}
          />

          {/* Response — between the input and the button */}
          {busy && (
            <Box sx={{ display: 'flex', justifyContent: 'center', py: 1 }}>
              <CircularProgress size={24} />
            </Box>
          )}
          {error && <Alert severity="error">{error}</Alert>}
          {done && !busy && <Alert severity="success">Account frozen</Alert>}

          {/* Execute */}
          <Button variant="contained" onClick={handleSubmit} disabled={busy}>
            {busy ? 'Loading…' : 'Send request'}
          </Button>
        </Stack>
      </Box>

      {/* Gray film + lock badge while collapsed (user-gated endpoint). */}
      <LockedFilm show={!expanded} symbol="lock" />
    </Paper>
  )
}
