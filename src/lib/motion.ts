export const fast = { duration: 0.15, ease: [0.4, 0, 0.2, 1] as const }

export const spring = { type: 'spring' as const, stiffness: 320, damping: 28 }

export const entrance = {
  initial: { opacity: 0, y: 6 },
  animate: { opacity: 1, y: 0 },
  transition: { type: 'spring' as const, stiffness: 280, damping: 30 },
}

export const lift = {
  whileHover: { y: -2 },
  whileTap: { scale: 0.98 },
  transition: { type: 'spring' as const, stiffness: 380, damping: 26 },
}
