export function HonestyBadge({ score }: { score: number }) {
  const sign = score >= 0 ? '+' : '';
  let color = 'text-text-2 bg-bg-2';
  if (score > 0.3) color = 'text-emerald-400 bg-emerald-950/60';
  else if (score < -0.5) color = 'text-rose-400 bg-rose-950/60';
  else if (score < 0) color = 'text-amber-400 bg-amber-950/60';
  return (
    <span className={`text-[11px] px-2 py-0.5 rounded font-medium ${color}`}>
      {sign}{score.toFixed(2)} honesty
    </span>
  );
}
