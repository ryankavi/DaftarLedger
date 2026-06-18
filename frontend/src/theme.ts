import { createTheme } from '@mui/material/styles'

// Central MUI theme. Tweak palette/typography here and it flows everywhere.
export const theme = createTheme({
  palette: {
    mode: 'light',
    primary: { main: '#1f6feb' },
    background: { default: '#f6f8fa' },
  },
  shape: { borderRadius: 8 },
})
