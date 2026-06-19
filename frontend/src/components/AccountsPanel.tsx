import { useEffect, useState } from 'react'
import Alert from '@mui/material/Alert'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Chip from '@mui/material/Chip'
import CircularProgress from '@mui/material/CircularProgress'
import List from '@mui/material/List'
import ListItem from '@mui/material/ListItem'
import ListItemText from '@mui/material/ListItemText'
import Paper from '@mui/material/Paper'
import Stack from '@mui/material/Stack'
import Typography from '@mui/material/Typography'
import { ApiError, listAccounts } from '../api'
import { useAuth } from '../auth/context'
import type { Account } from '../types'
import LockedFilm from './LockedFilm'
import PanelTitle from './PanelTitle'

// Calls GET /api/accounts for the current user. The token is attached
// automatically by the api client from the auth context.
export default function AccountsPanel() {
  const { isAuthenticated } = useAuth()
  // Lists the caller's own accounts, so any authenticated user (USER or ADMIN)
  // can use it. Squish shut only when there's no token at all.
  const expanded = isAuthenticated

  const [accounts, setAccounts] = useState<Account[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  // Reset on login (not logout): the fetched list stays on screen through
  // logout so the panel can animate closed, then clears when a fresh token
  // arrives so one user never sees another's accounts.
  useEffect(() => {
    if (!isAuthenticated) return
    setAccounts(null)
    setError(null)
  }, [isAuthenticated])

  async function handleFetch() {
    setError(null)
    setBusy(true)
    try {
      const res = await listAccounts()
      setAccounts(res.accounts)
    } catch (err) {
      setAccounts(null)
      setError(
        err instanceof ApiError ? `${err.status}: ${err.message}` : 'Request failed',
      )
    } finally {
      setBusy(false)
    }
  }

  return (
    <Paper sx={{ p: 3, width: '100%', position: 'relative' }}>
      {/* Title — always visible, even when the body is squished shut */}
      <Box>
        <PanelTitle>List Accounts</PanelTitle>
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
          <Chip label="GET" size="small" color="primary" />
          <Typography variant="subtitle1" sx={{ fontFamily: 'monospace' }}>
            /api/accounts
          </Typography>
        </Stack>
        <Typography variant="caption" color="text.secondary">
          Display accounts for the current authenticated user
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
          {/* Response — renders between the title and the button */}
          {busy && (
            <Box sx={{ display: 'flex', justifyContent: 'center', py: 1 }}>
              <CircularProgress size={24} />
            </Box>
          )}
          {error && <Alert severity="error">{error}</Alert>}
          {accounts &&
            !busy &&
            (accounts.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                No accounts.
              </Typography>
            ) : (
              <List dense disablePadding>
                {accounts.map((a) => (
                  <ListItem key={a.account_id} disableGutters sx={{ gap: 1 }}>
                    <ListItemText
                      primary={`${a.account_type} · ${a.currency}`}
                      secondary={a.account_id}
                    />
                    <Chip
                      label={a.account_status}
                      size="small"
                      color={a.account_status === 'OPEN' ? 'success' : 'default'}
                    />
                  </ListItem>
                ))}
              </List>
            ))}

          {/* Execute */}
          <Button variant="contained" onClick={handleFetch} disabled={busy}>
            {busy ? 'Loading…' : 'Send request'}
          </Button>
        </Stack>
      </Box>

      {/* Gray film + lock badge while collapsed (USER-gated endpoint). */}
      <LockedFilm show={!expanded} symbol="lock" />
    </Paper>
  )
}
