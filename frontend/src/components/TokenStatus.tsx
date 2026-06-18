import Chip from '@mui/material/Chip'
import { useAuth } from '../auth/context'

// Small live indicator of whether a token is loaded in the auth context. Re-renders
// automatically whenever the token changes (login sets it, logout clears it).
export default function TokenStatus() {
  const { isAuthenticated } = useAuth()

  if (isAuthenticated) {
    return <Chip label="Token loaded" color="primary" size="small" />
  }

  return (
    <Chip
      label="No token"
      variant="outlined"
      size="small"
      sx={{ color: 'rgba(255,255,255,0.7)', borderColor: 'rgba(255,255,255,0.3)' }}
    />
  )
}
