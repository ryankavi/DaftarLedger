import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Collapse from '@mui/material/Collapse'
import Divider from '@mui/material/Divider'
import Paper from '@mui/material/Paper'
import Typography from '@mui/material/Typography'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import MemoryOutlinedIcon from '@mui/icons-material/MemoryOutlined'

// Brand green (#6ba832) rendered as brushed metal: a diagonal run of light/dark
// green bands reads as light glancing off a metal bar.
const metallicGreen = {
  backgroundColor: '#6ba832',
  backgroundImage: `linear-gradient(160deg,
    #3f6a1d 0%,
    #6ba832 14%,
    #a6db78 31%,
    #6ba832 45%,
    #4f7d25 57%,
    #8bc25a 78%,
    #3f6a1d 100%)`,
}

// The Go service's distinguishing properties — one terse line each.
const FEATURES: { title: string; body: string }[] = [
  {
    title: 'Double-entry ledger',
    body: 'Every transaction posts balanced debit + credit entries — debits always equal credits.',
  },
  {
    title: 'ACID transactions',
    body: 'Each post runs in one SQL transaction: all rows commit together or roll back as one.',
  },
  {
    title: 'Idempotency',
    body: 'A unique idempotency key dedupes retries; a DB UNIQUE constraint backstops lost races.',
  },
  {
    title: 'Authorization layer',
    body: 'Stateless HS256 JWTs verified in middleware; role + ownership gate every protected route.',
  },
  {
    title: 'Deadlock-safe locking',
    body: 'Accounts locked SELECT … FOR UPDATE in canonical UUID order to prevent deadlocks.',
  },
  {
    title: 'Append-only & reversible',
    body: 'Ledger rows are never mutated or deleted; corrections post opposing reversal transactions.',
  },
  {
    title: 'Integer money',
    body: 'Balances stored as minor units (BIGINT) — never floats, so no rounding drift.',
  },
  {
    title: 'DB-authoritative time',
    body: 'posted_at is stamped by Postgres NOW(), not app clocks — immune to instance drift.',
  },
  {
    title: 'Auto-migrations',
    body: 'golang-migrate applies idempotent, versioned migrations automatically on startup.',
  },
  {
    title: 'Typed domain errors',
    body: 'Sentinel errors map to exact HTTP codes (400/403/404/409/422); 5xx bodies are masked.',
  },
]

const TECH = [
  'Go',
  'PostgreSQL 16',
  'database/sql',
  'lib/pq',
  'golang-migrate',
  'golang-jwt/jwt v5',
  'bcrypt',
  'net/http',
  'log/slog',
  'Docker',
  'testcontainers-go',
  'godotenv',
]

// Metallic-green toggle button. Lives in the top toolbar row alongside the token
// status + log-out controls; `open`/`onToggle` are owned by App so the dropdown
// (below) can render in a separate place in the layout.
export function BackendInfoButton({
  open,
  onToggle,
}: {
  open: boolean
  onToggle: () => void
}) {
  return (
    <Button
      onClick={onToggle}
      variant="contained"
      startIcon={<MemoryOutlinedIcon />}
      endIcon={
        <ExpandMoreIcon
          sx={{
            transform: open ? 'rotate(180deg)' : 'none',
            transition: 'transform 0.2s ease',
          }}
        />
      }
      sx={{
        ...metallicGreen,
        color: '#0d0d0d',
        fontWeight: 700,
        px: 2.5,
        borderRadius: 2,
        border: '1px solid rgba(255,255,255,0.3)',
        boxShadow:
          'inset 0 1px 0 rgba(255,255,255,0.5), 0 4px 14px rgba(0,0,0,0.4)',
        '&:hover': { ...metallicGreen, filter: 'brightness(1.06)' },
      }}
    >
      What the backend does
    </Button>
  )
}

// The drop-down summary panel: backend features as tiles, tech stack as bubbles.
// Rendered full-width and centered below the toolbar; visibility driven by `open`.
export default function BackendInfoPanel({ open }: { open: boolean }) {
  return (
    <Box sx={{ display: 'flex', justifyContent: 'center', px: 2, pb: 2 }}>
      <Collapse in={open} sx={{ width: '100%', maxWidth: 940 }}>
        <Paper
          elevation={0}
          sx={{
            ...metallicGreen,
            mt: 2,
            p: { xs: 2.5, md: 4 },
            borderRadius: 3,
            border: '1px solid rgba(255,255,255,0.25)',
            boxShadow:
              'inset 0 1px 0 rgba(255,255,255,0.45), inset 0 -2px 8px rgba(0,0,0,0.3), 0 12px 32px rgba(0,0,0,0.45)',
          }}
        >
          <Typography
            variant="h5"
            sx={{
              fontFamily: 'Poppins, sans-serif',
              fontWeight: 700,
              color: '#0d0d0d',
            }}
          >
            Under the Hood — Go Ledger Service
          </Typography>
          <Typography variant="body2" sx={{ color: 'rgba(13,13,13,0.78)', mb: 3 }}>
            The frontend is only UI. The correctness lives in the backend.
          </Typography>

          {/* Feature cards — dark translucent tiles so the text stays legible on
              the mid-tone metallic green. */}
          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)' },
              gap: 1.5,
            }}
          >
            {FEATURES.map((f) => (
              <Box
                key={f.title}
                sx={{
                  bgcolor: 'rgba(13,13,13,0.34)',
                  borderRadius: 2,
                  p: 1.75,
                  border: '1px solid rgba(255,255,255,0.12)',
                }}
              >
                <Typography sx={{ fontWeight: 700, color: '#d6f5b4', mb: 0.5 }}>
                  {f.title}
                </Typography>
                <Typography
                  variant="body2"
                  sx={{ color: 'rgba(255,255,255,0.92)', lineHeight: 1.45 }}
                >
                  {f.body}
                </Typography>
              </Box>
            ))}
          </Box>

          <Divider sx={{ my: 3, borderColor: 'rgba(13,13,13,0.25)' }} />

          {/* Tech stack — each item its own bubble. */}
          <Typography sx={{ fontWeight: 700, color: '#0d0d0d', mb: 1.5 }}>
            Tech stack
          </Typography>
          <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1 }}>
            {TECH.map((t) => (
              <Box
                key={t}
                sx={{
                  bgcolor: 'rgba(13,13,13,0.6)',
                  color: '#eaffd6',
                  px: 1.5,
                  py: 0.6,
                  borderRadius: 999,
                  fontSize: 13,
                  fontWeight: 600,
                  border: '1px solid rgba(255,255,255,0.15)',
                }}
              >
                {t}
              </Box>
            ))}
          </Box>
        </Paper>
      </Collapse>
    </Box>
  )
}
