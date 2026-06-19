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
import AccountBalanceOutlinedIcon from '@mui/icons-material/AccountBalanceOutlined'
import AccountBalanceWalletOutlinedIcon from '@mui/icons-material/AccountBalanceWalletOutlined'
import AttachMoneyIcon from '@mui/icons-material/AttachMoney'
import KeyOutlinedIcon from '@mui/icons-material/KeyOutlined'
import AutorenewIcon from '@mui/icons-material/Autorenew'
import { ApiError, deposit } from '../api'
import { useAuth } from '../auth/context'
import type { TransactionResponse } from '../types'
import LockedFilm from './LockedFilm'
import PanelTitle from './PanelTitle'

// Calls POST /api/transactions/deposit (EXTERNAL -> USER_CASH). The endpoint is
// admin-only on the backend (a non-admin POST gets a 403 in the error slot), but
// the panel is intentionally open to any authenticated user.
export default function DepositPanel() {
  const { isAuthenticated } = useAuth()
  // Not role-gated: any authenticated user sees the form. Squish shut only when
  // there's no token at all.
  const expanded = isAuthenticated

  const [fromAccountId, setFromAccountId] = useState('')
  const [toAccountId, setToAccountId] = useState('')
  const [amount, setAmount] = useState('')
  const [currency, setCurrency] = useState('USD')
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
    setFromAccountId('')
    setToAccountId('')
    setAmount('')
    setCurrency('USD')
    setIdempotencyKey(crypto.randomUUID())
    setResult(null)
    setError(null)
  }, [isAuthenticated])

  async function handleSubmit() {
    setError(null)
    setResult(null)
    setBusy(true)
    try {
      const res = await deposit({
        from_account_id: fromAccountId,
        to_account_id: toAccountId,
        amount: Math.round(Number(amount) * 100),
        currency,
        idempotency_key: idempotencyKey,
      })
      setResult(res)
      // Roll a fresh key so the next deposit isn't deduped against this one.
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
      {/* Title — always visible, even when the body is squished shut */}
      <Box>
        <PanelTitle>Deposit Cash</PanelTitle>
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
          <Chip label="POST" size="small" color="primary" />
          <Typography variant="subtitle1" sx={{ fontFamily: 'monospace' }}>
            /api/transactions/deposit
          </Typography>
        </Stack>
        <Typography variant="caption" color="text.secondary">
          Deposit cash: EXTERNAL → USER_CASH
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
          maxHeight: expanded ? '50rem' : 0,
          transform: expanded ? 'scaleY(1)' : 'scaleY(0)',
          opacity: expanded ? 1 : 0,
          pointerEvents: expanded ? 'auto' : 'none',
        }}
      >
        <Stack spacing={2} sx={{ mt: 2 }}>
          {/* Inputs */}
          <TextField
            variant="standard"
            label="From account (EXTERNAL)"
            value={fromAccountId}
            onChange={(e) => setFromAccountId(e.target.value)}
            fullWidth
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <AccountBalanceOutlinedIcon fontSize="small" />
                  </InputAdornment>
                ),
              },
            }}
          />
          <TextField
            variant="standard"
            label="To account (USER_CASH)"
            value={toAccountId}
            onChange={(e) => setToAccountId(e.target.value)}
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
          {/* Currency (short, $ icon) + Amount side by side */}
          <Stack direction="row" spacing={2} sx={{ alignItems: 'flex-end' }}>
            <TextField
              variant="standard"
              label="Currency"
              value={currency}
              onChange={(e) => setCurrency(e.target.value.toUpperCase())}
              sx={{ width: 110, flexShrink: 0 }}
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
            <TextField
              variant="standard"
              label="Amount (dollars)"
              type="number"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              sx={{ flex: 1 }}
              slotProps={{ htmlInput: { min: 0, step: 0.01 } }}
            />
          </Stack>
          <TextField
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

      {/* Gray film + lock badge while collapsed (open to any authenticated user). */}
      <LockedFilm show={!expanded} symbol="lock" />
    </Paper>
  )
}
