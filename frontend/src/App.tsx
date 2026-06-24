import { useState } from 'react'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Stack from '@mui/material/Stack'
import LoginForm from './components/LoginForm'
import SignupPanel from './components/SignupPanel'
import CreateAccountPanel from './components/CreateAccountPanel'
import AccountsPanel from './components/AccountsPanel'
import AccountBalancePanel from './components/AccountBalancePanel'
import CloseAccountPanel from './components/CloseAccountPanel'
import FreezeAccountPanel from './components/FreezeAccountPanel'
import ReopenAccountPanel from './components/ReopenAccountPanel'
import TransferPanel from './components/TransferPanel'
import WithdrawPanel from './components/WithdrawPanel'
import AssessFeePanel from './components/AssessFeePanel'
import RefundFeePanel from './components/RefundFeePanel'
import ReversePanel from './components/ReversePanel'
import DepositPanel from './components/DepositPanel'
import BackendInfoPanel, {
  BackendInfoButton,
} from './components/BackendInfoPanel'
import Header from './components/Header'
import TokenStatus from './components/TokenStatus'
import { useAuth } from './auth/context'

// Single page: login form, a live token-status chip + logout button, and the
// accounts endpoint panel. All share the auth context — log in and the token
// chip flips and the accounts request starts working.
export default function App() {
  const { logout } = useAuth()
  const [backendOpen, setBackendOpen] = useState(false)

  return (
    <Box sx={{ minHeight: '100%', display: 'flex', flexDirection: 'column' }}>
      {/* 3-cell row: empty left, centered button, right-pinned token + logout.
          The outer 1fr columns keep the button centered on the page regardless
          of the right group's width. */}
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: '1fr auto 1fr',
          alignItems: 'center',
          columnGap: 2,
          p: 2,
        }}
      >
        <Box sx={{ justifySelf: 'start' }}>
          <Header />
        </Box>
        <Box sx={{ justifySelf: 'center' }}>
          <BackendInfoButton
            open={backendOpen}
            onToggle={() => setBackendOpen((o) => !o)}
          />
        </Box>
        <Stack
          direction="row"
          spacing={2}
          sx={{ alignItems: 'center', justifySelf: 'end' }}
        >
          <TokenStatus />
          <Button variant="outlined" onClick={logout}>
            Log out
          </Button>
        </Stack>
      </Box>

      {/* Drop-down (toggled from the toolbar) summarizing backend features + stack. */}
      <BackendInfoPanel open={backendOpen} />

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
          gridTemplateColumns: { xs: '1fr', md: 'repeat(2, minmax(0, 441px))' },
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
        {/* Deposit spans the full width — its own double-width row. */}
        <Box sx={{ gridColumn: '1 / -1' }}>
          <DepositPanel />
        </Box>
        <CreateAccountPanel />
        <AccountsPanel />
        <AccountBalancePanel />
        <CloseAccountPanel />
        <FreezeAccountPanel />
        <ReopenAccountPanel />
        <TransferPanel />
        <WithdrawPanel />
        <AssessFeePanel />
        <RefundFeePanel />
        <ReversePanel />
      </Box>
    </Box>
  )
}
