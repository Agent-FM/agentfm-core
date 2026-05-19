import { forwardRef, ButtonHTMLAttributes, ReactNode } from 'react'
import { motion } from 'framer-motion'
import { lift } from '../../lib/motion'

export type ButtonVariant = 'default' | 'primary' | 'ghost' | 'danger'

interface Props extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'onAnimationStart' | 'onDragStart' | 'onDragEnd' | 'onDrag'> {
  variant?: ButtonVariant
  children: ReactNode
}

const VARIANT: Record<ButtonVariant, string> = {
  default: 'bg-bg-2 border border-border-1 text-text-1 hover:text-text-0 hover:border-accent/40 hover:shadow-[0_0_18px_-6px_rgba(34,211,238,.3)]',
  primary: 'relative text-accent-fg font-medium border border-accent/60 bg-gradient-to-br from-accent to-accent2 hover:from-accent2 hover:to-accent shadow-[0_0_0_1px_rgba(34,211,238,.4),0_8px_24px_-10px_rgba(34,211,238,.55)]',
  ghost:   'bg-transparent text-text-1 hover:text-accent',
  danger:  'bg-bad/15 border border-bad/40 text-bad hover:bg-bad/25 hover:border-bad/60',
}

export const Button = forwardRef<HTMLButtonElement, Props>(
  ({ variant = 'default', children, className, disabled, ...rest }, ref) => {
    return (
      <motion.button
        ref={ref}
        disabled={disabled}
        whileHover={disabled ? undefined : lift.whileHover}
        whileTap={disabled ? undefined : lift.whileTap}
        transition={lift.transition}
        className={`inline-flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-md text-xs transition-colors disabled:opacity-45 disabled:pointer-events-none ${VARIANT[variant]} ${className ?? ''}`}
        {...rest}
      >
        {children}
      </motion.button>
    )
  },
)
Button.displayName = 'Button'
