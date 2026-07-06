import { forwardRef, InputHTMLAttributes } from 'react'

type Props = InputHTMLAttributes<HTMLInputElement>

export const Input = forwardRef<HTMLInputElement, Props>(({ className, ...rest }, ref) => {
  return (
    <input
      ref={ref}
      className={`w-full bg-bg-0 border border-border-0 rounded-md px-2.5 py-1.5 text-sm text-text-0 placeholder-text-3 outline-none transition-shadow focus:border-accent focus:shadow-[0_0_0_3px_rgba(247,147,30,.15)] ${className ?? ''}`}
      {...rest}
    />
  )
})
Input.displayName = 'Input'
