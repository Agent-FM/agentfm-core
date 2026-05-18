export function CapabilityBadge({ name }: { name: string }) {
  return (
    <span className="text-[11px] px-2 py-0.5 rounded bg-bg-2 text-text-1 font-mono">
      {name}
    </span>
  );
}
