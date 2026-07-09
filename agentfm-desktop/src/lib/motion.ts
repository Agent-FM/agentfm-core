export const easeOut = [0.25, 1, 0.5, 1] as const

export const fast = { duration: 0.15, ease: easeOut }

export const spring = { duration: 0.18, ease: easeOut }

export const entrance = {
  initial: { opacity: 0 },
  animate: { opacity: 1 },
  transition: { duration: 0.15, ease: easeOut },
}

export const lift = {
  transition: { duration: 0.15, ease: easeOut },
}

export function staggerItem(_i: number) {
  return {
    initial: { opacity: 0 },
    animate: { opacity: 1 },
    transition: { duration: 0.15, ease: easeOut },
  }
}
