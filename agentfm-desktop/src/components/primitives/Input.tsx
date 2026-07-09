import { forwardRef, InputHTMLAttributes } from 'react'

type Props = InputHTMLAttributes<HTMLInputElement>

export const Input = forwardRef<HTMLInputElement, Props>(({ className, ...rest }, ref) => {
  return (
    <input
      ref={ref}
      className={`glass-inset w-full rounded-ctl h-[22px] px-2 text-sm text-text-0 placeholder-text-2 outline-none transition-shadow focus:border-accent/60 focus:shadow-[0_0_0_2px_rgb(var(--accent)/0.35)] ${className ?? ''}`}
      {...rest}
    />
  )
})
Input.displayName = 'Input'
