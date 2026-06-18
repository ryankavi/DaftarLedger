import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Card from '@mui/material/Card'
import CardContent from '@mui/material/CardContent'
import Stack from '@mui/material/Stack'
import Typography from '@mui/material/Typography'
import LoginForm from './components/LoginForm'
import { useAuth } from './auth/context'

// Thin shell: pick the view based on auth state. Detailed UI lives in its own
// components (LoginForm now; real screens — accounts, transactions — next).
export default function App() {
  const { isAuthenticated, token, logout } = useAuth()

  if (!isAuthenticated) {
    return <LoginForm />
  }

  // Temporary signed-in placeholder — replaced by real screens next.
  return (
    <Box sx={{ minHeight: '100%', display: 'grid', placeItems: 'center', p: 2 }}>
      <Card sx={{ maxWidth: 520, width: '100%' }}>
        <CardContent>
          <Stack spacing={2}>
            <Typography variant="h6">Signed in</Typography>
            <Typography
              variant="body2"
              color="text.secondary"
              sx={{ wordBreak: 'break-all' }}
            >
              Token: {token}
            </Typography>
            <Button variant="outlined" onClick={logout}>
              Log out
            </Button>
          </Stack>
        </CardContent>
      </Card>
    </Box>
  )
}
