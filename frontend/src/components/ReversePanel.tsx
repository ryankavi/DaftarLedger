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
import ClearableTextField from './ClearableTextField'
import Typography from '@mui/material/Typography'
import ReceiptLongOutlinedIcon from '@mui/icons-material/ReceiptLongOutlined'
import NotesOutlinedIcon from '@mui/icons-material/NotesOutlined'
import KeyOutlinedIcon from '@mui/icons-material/KeyOutlined'
import AutorenewIcon from '@mui/icons-material/Autorenew'
import { ApiError, reverseTransaction } from '../api'
import { useAuth } from '../auth/context'
import type { TransactionResponse } from '../types'
import LockedFilm from './LockedFilm'
import PanelTitle from './PanelTitle'
import HelpPopover from './HelpPopover'

// Calls POST /api/transactions/{id}/reverse. Admin-only: a non-admin token gets
// a 403 surfaced in the error slot.
export default function ReversePanel() {
  const { isAuthenticated, role } = useAuth()
  // Admin-only platform flow, so the panel only stays open for an ADMIN token.
  const expanded = isAuthenticated && role === 'ADMIN'

  const [transactionId, setTransactionId] = useState('')
  const [memo, setMemo] = useState('')
  const [idempotencyKey, setIdempotencyKey] = useState<string>(() =>
    crypto.randomUUID(),
  )
  const [result, setResult] = useState<TransactionResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  // Reset on login (not logout): the last response stays on screen through
  // logout so the panel can animate closed with content to squish, then clears
  // when a fresh token arrives.
  useEffect(() => {
    if (!isAuthenticated) return
    setTransactionId('')
    setMemo('')
    setIdempotencyKey(crypto.randomUUID())
    setResult(null)
    setError(null)
  }, [isAuthenticated])

  async function handleSubmit() {
    setError(null)
    setResult(null)
    setBusy(true)
    try {
      const res = await reverseTransaction(transactionId, {
        idempotency_key: idempotencyKey,
        memo: memo || undefined,
      })
      setResult(res)
      // Roll a fresh key so the next reversal isn't deduped against this one.
      setIdempotencyKey(crypto.randomUUID())
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
          Reverse a posted transaction by id, creating an opposing set of
          entries. Optionally include a memo. Admin only.
        </p>
      </HelpPopover>

      {/* Title — always visible, even when the body is squished shut */}
      <Box>
        <PanelTitle>Reverse Transaction</PanelTitle>
        <Stack
          direction="row"
          spacing={1}
          sx={{ alignItems: 'center', flexWrap: 'wrap' }}
        >
          <Chip label="POST" size="small" color="primary" />
          <Typography variant="subtitle1" sx={{ fontFamily: 'monospace' }}>
            {'/api/transactions/{id}/reverse'}
          </Typography>
          <Chip label="ADMIN" size="small" color="warning" />
        </Stack>
        <Typography variant="caption" color="text.secondary">
          Reverse a posted transaction with opposing entries (admin only)
        </Typography>
      </Box>

      {/* Body — collapses with a vertical squish when the caller isn't an admin. */}
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
            label="Transaction ID"
            value={transactionId}
            onChange={(e) => setTransactionId(e.target.value)}
            fullWidth
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <ReceiptLongOutlinedIcon fontSize="small" />
                  </InputAdornment>
                ),
              },
            }}
          />
          <ClearableTextField
            variant="standard"
            label="Memo (optional)"
            value={memo}
            onChange={(e) => setMemo(e.target.value)}
            fullWidth
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <NotesOutlinedIcon fontSize="small" />
                  </InputAdornment>
                ),
              },
            }}
          />
          <ClearableTextField
            variant="standard"
            label="Idempotency key"
            value={idempotencyKey}
            onChange={(e) => setIdempotencyKey(e.target.value)}
            fullWidth
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <KeyOutlinedIcon fontSize="small" />
                  </InputAdornment>
                ),
                endAdornment: (
                  <InputAdornment position="end">
                    <IconButton
                      onClick={() => setIdempotencyKey(crypto.randomUUID())}
                      edge="end"
                      size="small"
                      aria-label="Randomize idempotency key"
                    >
                      <AutorenewIcon fontSize="small" />
                    </IconButton>
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
              {result.transaction_type} · {result.transaction_status}
              <br />
              {result.transaction_id}
            </Alert>
          )}

          {/* Execute */}
          <Button variant="contained" onClick={handleSubmit} disabled={busy}>
            {busy ? 'Loading…' : 'Send request'}
          </Button>
        </Stack>
      </Box>

      {/* Gray film + shield badge while collapsed (ADMIN-only endpoint). */}
      <LockedFilm show={!expanded} symbol="shield" />
    </Paper>
  )
}
