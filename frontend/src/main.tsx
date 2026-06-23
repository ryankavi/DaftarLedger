import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { ThemeProvider } from '@mui/material/styles'
import CssBaseline from '@mui/material/CssBaseline'
// Latin + latin-ext subsets only. The unscoped @fontsource/<font>/<weight>.css
// imports pull in every script (cyrillic, greek, devanagari, vietnamese, …),
// none of which this English UI renders — that bloated dist/ with ~64 woff
// files no browser ever downloads. latin-ext is kept for accented chars
// (é, ñ, ü, ł) in user names/emails.
import '@fontsource/roboto/latin-300.css'
import '@fontsource/roboto/latin-ext-300.css'
import '@fontsource/roboto/latin-400.css'
import '@fontsource/roboto/latin-ext-400.css'
import '@fontsource/roboto/latin-500.css'
import '@fontsource/roboto/latin-ext-500.css'
import '@fontsource/roboto/latin-700.css'
import '@fontsource/roboto/latin-ext-700.css'
import '@fontsource/poppins/latin-600.css'
import '@fontsource/poppins/latin-ext-600.css'
import '@fontsource/poppins/latin-700.css'
import '@fontsource/poppins/latin-ext-700.css'
import './index.css'
import { theme } from './theme'
import { AuthProvider } from './auth/AuthProvider'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <AuthProvider>
        <App />
      </AuthProvider>
    </ThemeProvider>
  </StrictMode>,
)
