import { ButtonHTMLAttributes, forwardRef } from 'react'
import { motion } from 'framer-motion'

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'default' | 'primary' | 'ghost'
}

export const Button = forwardRef<HTMLButtonElement, Props>(
  ({ variant = 'default', className = '', children, disabled, ...rest }, ref) => {
    const base =
      'inline-flex items-center justify-center text-xs px-3 py-1.5 rounded-md transition-colors disabled:opacity-50 disabled:cursor-not-allowed'
    const styles: Record<NonNullable<Props['variant']>, string> = {
      default: 'bg-bg-2 border border-border-0 text-text-0 hover:bg-bg-2/80',
      primary: 'bg-accent text-accent-fg font-medium hover:opacity-90',
      ghost: 'text-text-2 hover:text-text-0',
    }
    return (
      <motion.button
        ref={ref}
        whileTap={disabled ? undefined : { scale: 0.98 }}
        disabled={disabled}
        className={`${base} ${styles[variant ?? 'default']} ${className}`}
        {...rest}
      >
        {children}
      </motion.button>
    )
  },
)
Button.displayName = 'Button'
