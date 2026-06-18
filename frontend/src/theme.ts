import { createTheme } from '@mui/material/styles'

// Central MUI theme. Tweak palette/typography here and it flows everywhere.
//
// Brand: #6ba832 green as primary, near-black + gray as the supporting accents.
// Note: #6ba832 is mid-tone, so white text on it fails WCAG (~2.6:1) while
// near-black passes (~8:1) — hence contrastText is dark, which also gives the
// black-on-green "highlight" look.
export const theme = createTheme({
  palette: {
    mode: 'light',
    primary: {
      main: '#6ba832',
      light: '#8bc25a',
      dark: '#4f7d25',
      contrastText: '#0d0d0d',
    },
    secondary: {
      main: '#2b2b2b',
      light: '#4a4a4a',
      dark: '#161616',
      contrastText: '#ffffff',
    },
    text: {
      primary: '#1a1a1a',
      secondary: '#5f6368',
    },
    background: {
      // Charcoal page: green-on-green washed out the card's glow, so the backdrop
      // is neutral. Cards (paper) stay white to pop against it.
      default: '#1c1d18',
      paper: '#ffffff',
    },
    divider: '#e1e3df',
  },
  shape: { borderRadius: 8 },
  typography: {
    // Flatter, less shouty buttons than the MUI default ALL-CAPS.
    button: { textTransform: 'none', fontWeight: 600 },
  },
})
