import type { ButtonHTMLAttributes, ReactNode } from 'react'

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'cyan' | 'violet'
  children: ReactNode
}

export function GradientButton({ variant = 'cyan', className = '', children, ...rest }: Props) {
  const gradient = variant === 'cyan'
    ? 'linear-gradient(135deg, #22d3ee, #06b6d4)'
    : 'linear-gradient(135deg, #a855f7, #7e22ce)'
  const shadow = variant === 'cyan'
    ? '0 4px 14px -4px rgba(34,211,238,.5)'
    : '0 4px 14px -4px rgba(168,85,247,.5)'
  const fg = variant === 'cyan' ? '#07090d' : '#ffffff'
  return (
    <button
      {...rest}
      className={`inline-flex items-center justify-center gap-1.5 px-4 py-2.5
        rounded-xl font-semibold text-[13px] border-0 transition-all
        disabled:opacity-50 disabled:cursor-not-allowed
        hover:brightness-110 ${className}`}
      style={{ background: gradient, color: fg, boxShadow: shadow }}
    >
      {children}
    </button>
  )
}
