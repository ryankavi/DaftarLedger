import { keyframes } from '@mui/material/styles'

// Breathing glow: a brand-green halo that brightens and dims, drawing the eye
// without nagging. Shared by the login form and the create-account panel.
export const breathe = keyframes`
  0%, 100% { box-shadow: 0 0 6px 0 rgba(139, 194, 90, 0.5); }
  50%      { box-shadow: 0 0 22px 4px rgba(139, 194, 90, 0.9); }
`

// Drop-in sx for a glowing green border. Spread into a component's sx.
export const glowBorderSx = {
  border: '1px solid',
  borderColor: 'primary.light',
  animation: `${breathe} 2.8s ease-in-out infinite`,
  // Respect users who prefer reduced motion.
  '@media (prefers-reduced-motion: reduce)': { animation: 'none' },
}
