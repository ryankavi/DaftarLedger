import Box from '@mui/material/Box'
import LockIcon from '@mui/icons-material/Lock'
import ShieldIcon from '@mui/icons-material/Shield'

// Translucent gray film laid over a collapsed (access-gated) panel, badged with
// a symbol that hints why it's locked: a lock for user-gated endpoints, a shield
// for admin-only ones. Fades in/out with the panel's squish, and never eats
// pointer events so the expanded form underneath stays interactive.
export default function LockedFilm({
  show,
  symbol,
}: {
  show: boolean
  symbol: 'lock' | 'shield'
}) {
  const Icon = symbol === 'shield' ? ShieldIcon : LockIcon
  return (
    <Box
      aria-hidden
      sx={{
        position: 'absolute',
        inset: 0,
        borderRadius: 1,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        bgcolor: 'rgba(20, 15, 15, 0.6)',
        opacity: show ? 1 : 0,
        pointerEvents: 'none',
        transition: (theme) =>
          theme.transitions.create('opacity', {
            duration: theme.transitions.duration.standard,
          }),
      }}
    >
      <Icon sx={{ color: 'primary.main', fontSize: 40 }} />
    </Box>
  )
}
