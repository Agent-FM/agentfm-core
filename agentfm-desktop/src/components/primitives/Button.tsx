import { forwardRef, ButtonHTMLAttributes, ReactNode } from 'react'

export type ButtonVariant = 'default' | 'primary' | 'secondary' | 'ghost' | 'danger' | 'outline'

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  children: ReactNode
}

const VARIANT: Record<ButtonVariant, string> = {
  default:   'bg-control text-text-0 hover:bg-control-hover active:bg-control-active',
  secondary: 'bg-control text-text-0 hover:bg-control-hover active:bg-control-active',
  primary:   'bg-accent text-accent-fg font-medium hover:bg-accent-light active:bg-accent-dim',
  outline:   'bg-transparent border border-border-1 text-text-0 hover:bg-white/[0.05]',
  ghost:     'bg-transparent text-text-1 hover:text-text-0 hover:bg-white/[0.05]',
  danger:    'bg-bad/15 border border-bad/40 text-bad hover:bg-bad/25',
}

export const Button = forwardRef<HTMLButtonElement, Props>(
  ({ variant = 'default', children, className, disabled, ...rest }, ref) => {
    return (
      <button
        ref={ref}
        disabled={disabled}
        className={`inline-flex items-center justify-center gap-1.5 h-[22px] px-2.5 rounded-ctl text-sm transition-colors duration-150 disabled:opacity-45 disabled:pointer-events-none cursor-pointer ${VARIANT[variant]} ${className ?? ''}`}
        {...rest}
      >
        {children}
      </button>
    )
  },
)
Button.displayName = 'Button'
