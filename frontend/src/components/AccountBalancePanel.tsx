import { useEffect, useState } from 'react'
import Alert from '@mui/material/Alert'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Chip from '@mui/material/Chip'
import CircularProgress from '@mui/material/CircularProgress'
import InputAdornment from '@mui/material/InputAdornment'
import Paper from '@mui/material/Paper'
import Stack from '@mui/material/Stack'
import ClearableTextField from './ClearableTextField'
import Typography from '@mui/material/Typography'
import AccountBalanceWalletOutlinedIcon from '@mui/icons-material/AccountBalanceWalletOutlined'
import ScheduleOutlinedIcon from '@mui/icons-material/ScheduleOutlined'
import { ApiError, getAccountBalance } from '../api'
import { useAuth } from '../auth/context'
import type { AccountBalanceResponse } from '../types'
import LockedFilm from './LockedFilm'
import PanelTitle from './PanelTitle'
import HelpPopover from './HelpPopover'

// Calls GET /api/accounts/{id}/balance for the current user. Any authenticated
// user may read their own account's balance, so it's user-gated (lock film).
export default function AccountBalancePanel() {
  const { isAuthenticated } = useAuth()
  const expanded = isAuthenticated

  const [accountId, setAccountId] = useState('')
  const [asOf, setAsOf] = useState('')
  const [result, setResult] = useState<AccountBalanceResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  // Reset on login (not logout): keep the last result on screen through logout
  // so the panel animates closed, then clear when a fresh token arrives.
  useEffect(() => {
    if (!isAuthenticated) return
    setAccountId('')
    setAsOf('')
    setResult(null)
    setError(null)
  }, [isAuthenticated])

  async function handleFetch() {
    setError(null)
    setResult(null)
    setBusy(true)
    try {
      const res = await getAccountBalance(accountId, asOf || undefined)
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
    <Paper sx={{ p: 3, width: '100%', position: 'relative' }}>
      <HelpPopover>
        <p>
          Look up an account's net balance. Optionally pass an
          "as of" timestamp to see the balance at a point in time.
        </p>
      </HelpPopover>

      {/* Title — always visible, even when the body is squished shut */}
      <Box>
        <PanelTitle>Account Balance</PanelTitle>
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
          <Chip label="GET" size="small" color="primary" />
          <Typography variant="subtitle1" sx={{ fontFamily: 'monospace' }}>
            {'/api/accounts/{id}/balance'}
          </Typography>
        </Stack>
        <Typography variant="caption" color="text.secondary">
          Net balance of account, optionally as of a timestamp
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
          <ClearableTextField
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
          <ClearableTextField
            variant="standard"
            label="As of (optional)"
            value={asOf}
            onChange={(e) => setAsOf(e.target.value)}
            fullWidth
            helperText="RFC 3339, e.g. 2026-06-19T00:00:00Z"
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <ScheduleOutlinedIcon fontSize="small" />
                  </InputAdornment>
                ),
              },
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
              Balance: ${(result.balance / 100).toFixed(2)}
            </Alert>
          )}

          {/* Execute */}
          <Button variant="contained" onClick={handleFetch} disabled={busy}>
            {busy ? 'Loading…' : 'Send request'}
          </Button>
        </Stack>
      </Box>

      {/* Gray film + lock badge while collapsed (user-gated endpoint). */}
      <LockedFilm show={!expanded} symbol="lock" />
    </Paper>
  )
}
