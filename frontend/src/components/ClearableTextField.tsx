import type { ChangeEvent, ReactNode } from 'react'
import TextField, { type TextFieldProps } from '@mui/material/TextField'
import IconButton from '@mui/material/IconButton'
import InputAdornment from '@mui/material/InputAdornment'
import ClearIcon from '@mui/icons-material/Clear'

// Drop-in replacement for MUI's TextField that renders a small "x" clear button
// at the right edge whenever the field has content. It clears by dispatching an
// empty-value change through the field's own onChange, so call sites only need to
// pass value + onChange (no extra onClear prop). Any existing endAdornment — a
// password visibility toggle, the idempotency-key randomize button — is preserved;
// the clear button sits just to its left.
export default function ClearableTextField(props: TextFieldProps) {
  const { value, onChange, slotProps, disabled, ...rest } = props

  const hasValue = value != null && value !== ''

  // Every call site in this app passes slotProps.input as an object literal.
  const inputProps =
    typeof slotProps?.input === 'object' && slotProps.input !== null
      ? (slotProps.input as Record<string, unknown>)
      : {}
  const existingEnd = inputProps.endAdornment as ReactNode

  const clearAdornment =
    hasValue && !disabled ? (
      <InputAdornment position="end">
        <IconButton
          onClick={() =>
            onChange?.({
              target: { value: '' },
            } as ChangeEvent<HTMLInputElement>)
          }
          edge={existingEnd ? false : 'end'}
          size="small"
          aria-label="Clear field"
          tabIndex={-1}
        >
          <ClearIcon fontSize="small" />
        </IconButton>
      </InputAdornment>
    ) : null

  const endAdornment =
    clearAdornment || existingEnd ? (
      <>
        {clearAdornment}
        {existingEnd}
      </>
    ) : undefined

  return (
    <TextField
      {...rest}
      value={value}
      onChange={onChange}
      disabled={disabled}
      slotProps={
        {
          ...slotProps,
          input: {
            ...inputProps,
            endAdornment,
          },
        } as TextFieldProps['slotProps']
      }
    />
  )
}
