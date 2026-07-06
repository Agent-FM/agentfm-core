export const expoOut = [0.16, 1, 0.3, 1] as const

export const fast = { duration: 0.18, ease: expoOut }

export const spring = { type: 'spring' as const, stiffness: 320, damping: 30 }

export const entrance = {
  initial: { opacity: 0, y: 6 },
  animate: { opacity: 1, y: 0 },
  transition: { duration: 0.32, ease: expoOut },
}

export const lift = {
  whileHover: { y: -2 },
  whileTap: { scale: 0.97 },
  transition: { type: 'spring' as const, stiffness: 320, damping: 30 },
}

export function staggerItem(i: number) {
  return {
    initial: { opacity: 0, y: 8 },
    animate: { opacity: 1, y: 0 },
    transition: { duration: 0.3, ease: expoOut, delay: i * 0.04 },
  }
}
