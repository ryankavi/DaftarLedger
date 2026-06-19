import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Stack from '@mui/material/Stack'
import LoginForm from './components/LoginForm'
import SignupPanel from './components/SignupPanel'
import CreateAccountPanel from './components/CreateAccountPanel'
import AccountsPanel from './components/AccountsPanel'
import DepositPanel from './components/DepositPanel'
import TokenStatus from './components/TokenStatus'
import { useAuth } from './auth/context'

// Single page: login form, a live token-status chip + logout button, and the
// accounts endpoint panel. All share the auth context — log in and the token
// chip flips and the accounts request starts working.
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

      {/* 2-column grid of fixed-width columns, centered as a group so the only
          gap between panels is `gap` (not half the viewport). Default row-major
          flow fills left, then right, then wraps to the next row.

          Deliberately NOT keyed on logoutNonce: remounting on logout would snap
          the squish panels shut instead of letting them animate closed. Each
          panel now resets its own state instead (on login for the squish
          panels, on logout for the auth panels). */}
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: { xs: '1fr', md: 'repeat(2, minmax(0, 400px))' },
          justifyContent: 'center',
          rowGap: 3,
          columnGap: 5,
          alignItems: 'start',
          px: 2,
          pb: 6,
        }}
      >
        <LoginForm />
        <SignupPanel />
        <CreateAccountPanel />
        <AccountsPanel />
        <DepositPanel />
      </Box>
    </Box>
  )
}
