export function starsFromScore(score: number): number {
  const clamped = Math.max(-1, Math.min(1, score))
  const halves = (clamped + 1) * 5
  const floor = Math.floor(halves)
  const diff = halves - floor
  let rounded: number
  if (diff > 0.5) {
    rounded = floor + 1
  } else if (diff < 0.5) {
    rounded = floor
  } else {
    rounded = floor % 2 === 0 ? floor : floor + 1
  }
  return rounded / 2
}
