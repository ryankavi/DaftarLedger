import { useState, type ReactNode } from 'react'
import Box from '@mui/material/Box'
import IconButton from '@mui/material/IconButton'
import Popover from '@mui/material/Popover'
import HelpOutlineIcon from '@mui/icons-material/HelpOutlined'

// Shared "?" help button pinned to a panel's top-right corner. Clicking it opens
// a popover with whatever content the panel passes as children. Per panel you
// only supply the text — write paragraphs as <p>…</p> and they get consistent
// styling here. The host panel's <Paper> must be position: relative (the squish
// panels already are).
export default function HelpPopover({ children }: { children: ReactNode }) {
  const [anchorEl, setAnchorEl] = useState<HTMLButtonElement | null>(null)
  const open = Boolean(anchorEl)

  return (
    <>
      <IconButton
        aria-label="Help"
        size="small"
        onClick={(e) => setAnchorEl(e.currentTarget)}
        sx={{ position: 'absolute', top: 8, right: 8, zIndex: 3 }}
      >
        <HelpOutlineIcon fontSize="small" />
      </IconButton>
      <Popover
        open={open}
        anchorEl={anchorEl}
        onClose={() => setAnchorEl(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
      >
        <Box
          sx={{
            p: 2,
            maxWidth: 320,
            '& p': { m: 0, mb: 1, fontSize: '0.875rem', lineHeight: 1.5 },
            '& p:last-child': { mb: 0 },
          }}
        >
          {children}
        </Box>
      </Popover>
    </>
  )
}
