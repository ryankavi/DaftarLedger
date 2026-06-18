import { useState } from 'react'
import Alert from '@mui/material/Alert'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Chip from '@mui/material/Chip'
import CircularProgress from '@mui/material/CircularProgress'
import Paper from '@mui/material/Paper'
import Stack from '@mui/material/Stack'
import TextField from '@mui/material/TextField'
import Typography from '@mui/material/Typography'
import { ApiError, deposit } from '../api'
import type { TransactionResponse } from '../types'

// Calls POST /api/transactions/deposit (EXTERNAL -> USER_CASH). Admin-only: a
// non-admin token gets a 403 surfaced in the error slot.
export default function DepositPanel() {
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

  async function handleSubmit() {
    setError(null)
    setResult(null)
    setBusy(true)
    try {
      const res = await deposit({
        from_account_id: fromAccountId,
        to_account_id: toAccountId,
        amount: Number(amount),
        currency,
        idempotency_key: idempotencyKey,
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
    <Paper sx={{ p: 3, width: '100%' }}>
      <Stack spacing={2}>
        {/* Title */}
        <Box>
          <Stack
            direction="row"
            spacing={1}
            sx={{ alignItems: 'center', flexWrap: 'wrap' }}
          >
            <Chip label="POST" size="small" color="primary" />
            <Typography variant="subtitle1" sx={{ fontFamily: 'monospace' }}>
              /api/transactions/deposit
            </Typography>
            <Chip label="ADMIN" size="small" color="warning" />
          </Stack>
          <Typography variant="caption" color="text.secondary">
            Deposit cash: EXTERNAL → USER_CASH (admin only)
          </Typography>
        </Box>

        {/* Inputs */}
        <TextField
          label="From account (EXTERNAL)"
          value={fromAccountId}
          onChange={(e) => setFromAccountId(e.target.value)}
          size="small"
          fullWidth
        />
        <TextField
          label="To account (USER_CASH)"
          value={toAccountId}
          onChange={(e) => setToAccountId(e.target.value)}
          size="small"
          fullWidth
        />
        <TextField
          label="Amount (minor units)"
          type="number"
          value={amount}
          onChange={(e) => setAmount(e.target.value)}
          size="small"
          fullWidth
          slotProps={{ htmlInput: { min: 1 } }}
        />
        <TextField
          label="Currency"
          value={currency}
          onChange={(e) => setCurrency(e.target.value.toUpperCase())}
          size="small"
          fullWidth
          slotProps={{ htmlInput: { maxLength: 3 } }}
        />
        {/* Field + Randomize button, attached as one input group. */}
        <Box>
          <TextField
            label="Idempotency key"
            value={idempotencyKey}
            onChange={(e) => setIdempotencyKey(e.target.value)}
            size="small"
            fullWidth
            sx={{
              '& .MuiOutlinedInput-root': {
                borderBottomLeftRadius: 0,
                borderBottomRightRadius: 0,
              },
            }}
          />
          <Button
            variant="outlined"
            fullWidth
            onClick={() => setIdempotencyKey(crypto.randomUUID())}
            sx={{
              borderTopLeftRadius: 0,
              borderTopRightRadius: 0,
              mt: '-1px',
            }}
          >
            Randomize
          </Button>
        </Box>

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
    </Paper>
  )
}
