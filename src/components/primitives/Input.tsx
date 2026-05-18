import { InputHTMLAttributes, forwardRef } from 'react'

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  ({ className = '', ...rest }, ref) => (
    <input
      ref={ref}
      className={`bg-bg-0 border border-border-0 rounded-md px-3 py-1.5 text-xs text-text-0 outline-none focus:border-accent transition-colors ${className}`}
      {...rest}
    />
  ),
)
Input.displayName = 'Input'
