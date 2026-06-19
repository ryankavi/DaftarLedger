import Typography from '@mui/material/Typography'
import type { ReactNode } from 'react'

// Bold, human-readable panel heading in Poppins (a different face than the
// Roboto body font), describing what the panel does. Sits above the
// method/endpoint chips.
export default function PanelTitle({ children }: { children: ReactNode }) {
  return (
    <Typography
      sx={{
        fontFamily: '"Poppins", sans-serif',
        fontWeight: 700,
        fontSize: '1.15rem',
        lineHeight: 1.3,
        mb: 0.5,
      }}
    >
      {children}
    </Typography>
  )
}
