export function DispatchBadge({ allowed, reason }: { allowed: boolean; reason?: string }) {
  if (allowed) {
    return (
      <span className="text-[11px] px-2 py-0.5 rounded text-emerald-400 bg-emerald-950/60 font-medium">
        ✓ allowed
      </span>
    );
  }
  return (
    <span
      className="text-[11px] px-2 py-0.5 rounded text-rose-400 bg-rose-950/60 font-medium"
      title={reason}
    >
      ✗ refused
    </span>
  );
}
