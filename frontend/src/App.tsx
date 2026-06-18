import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Stack from '@mui/material/Stack'
import LoginForm from './components/LoginForm'
import TokenStatus from './components/TokenStatus'
import { useAuth } from './auth/context'

// Single page: the login form plus a live token-status chip and a logout button.
// All three share the auth context, so logging in or clicking "Log out" updates
// the chip automatically — no view swapping.
export default function App() {
  const { logout } = useAuth()

  return (
    <Box sx={{ minHeight: '100%', display: 'flex', flexDirection: 'column' }}>
      <Stack
        direction="row"
        spacing={2}
        sx={{ alignItems: 'center', justifyContent: 'flex-end', p: 2 }}
      >
        <TokenStatus />
        <Button variant="outlined" onClick={logout}>
          Log out
        </Button>
      </Stack>

      <Box sx={{ flex: 1 }}>
        <LoginForm />
      </Box>
    </Box>
  )
}
