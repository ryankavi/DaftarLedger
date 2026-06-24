import Box from '@mui/material/Box'
import Divider from '@mui/material/Divider'
import Typography from '@mui/material/Typography'

// Grey-metallic sheen applied to text: the brushed-metal gradient is clipped to
// the glyphs (background-clip: text + transparent fill) so the letters read as
// polished silver. Same band sequence as the old silver buttons.
const silverSheen = {
  backgroundImage: `linear-gradient(160deg,
    #9a9da3 0%,
    #e8eaee 14%,
    #ffffff 31%,
    #d6d8dd 45%,
    #b6b9c0 57%,
    #f2f3f6 78%,
    #9a9da3 100%)`,
  WebkitBackgroundClip: 'text',
  backgroundClip: 'text',
  color: 'transparent',
  WebkitTextFillColor: 'transparent',
}

// Site header: the "Daftar / ledger" wordmark in silver, with the project
// tagline pinned to its right. Lives in the left cell of App's toolbar row.
export default function Header() {
  return (
    <Box
      component="header"
      sx={{ display: 'flex', alignItems: 'center', gap: 2 }}
    >
      <Box>
        <Typography
          sx={{
            ...silverSheen,
            fontFamily: 'Poppins, sans-serif',
            fontWeight: 700,
            fontSize: 28,
            lineHeight: 1,
          }}
        >
          Daftar
        </Typography>
        <Typography
          sx={{
            ...silverSheen,
            fontFamily: 'Poppins, sans-serif',
            fontWeight: 400,
            fontSize: 22,
            lineHeight: 1.1,
          }}
        >
          ledger
        </Typography>
      </Box>

      <Divider
        orientation="vertical"
        flexItem
        sx={{ borderColor: 'rgba(255,255,255,0.2)' }}
      />

      <Typography
        sx={{
          color: 'rgba(255,255,255,0.7)',
          fontSize: 14,
          maxWidth: 180,
          lineHeight: 1.3,
        }}
      >
        a mock double-entry ledger written in Go
      </Typography>
    </Box>
  )
}
