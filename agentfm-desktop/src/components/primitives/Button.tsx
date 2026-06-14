import { forwardRef, ButtonHTMLAttributes, ReactNode } from 'react'
import { motion } from 'framer-motion'
import { lift } from '../../lib/motion'

export type ButtonVariant = 'default' | 'primary' | 'secondary' | 'ghost' | 'danger'

interface Props extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'onAnimationStart' | 'onDragStart' | 'onDragEnd' | 'onDrag'> {
  variant?: ButtonVariant
  children: ReactNode
}

const VARIANT: Record<ButtonVariant, string> = {
  default:   'bg-bg-1 border border-border-0 text-text-1 hover:text-text-0 hover:border-border-1',
  secondary: 'bg-bg-1 border border-border-0 text-text-1 hover:text-text-0 hover:border-border-1',
  primary:   'bg-accent text-accent-fg font-medium hover:bg-accent-dim',
  ghost:     'bg-transparent text-text-1 hover:text-text-0',
  danger:    'bg-bad/15 border border-bad/40 text-bad hover:bg-bad/25 hover:border-bad/60',
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
        className={`inline-flex items-center justify-center gap-1.5 px-3.5 py-2 rounded-[10px] text-sm transition-colors disabled:opacity-45 disabled:pointer-events-none ${VARIANT[variant]} ${className ?? ''}`}
        {...rest}
      >
        {children}
      </motion.button>
    )
  },
)
Button.displayName = 'Button'
