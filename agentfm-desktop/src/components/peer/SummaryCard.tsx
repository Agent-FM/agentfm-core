import type { PeerSummary } from '../../types/api';
import { shortenDigest, compactAge } from '../../lib/peer';
import { Badge } from '../primitives/Badge';

export function SummaryCard({ data }: { data: PeerSummary }) {
  const score = data.honesty_score;
  return (
    <div className="bg-bg-1 border border-border-0 rounded-lg p-5 grid grid-cols-3 gap-x-6 gap-y-4">
      <Field label="Rating">
        <div className="flex gap-1.5 flex-wrap items-center">
          {data.is_equivocator ? (
            <Badge tone="rose">⚠ equivocator</Badge>
          ) : (
            <Badge tone={score > 0.3 ? 'lime' : score < -0.5 ? 'rose' : 'neutral'} mono>
              {score >= 0 ? '+' : ''}{score.toFixed(2)}
            </Badge>
          )}
        </div>
      </Field>
      <Field label="Status">
        {data.online ? (
          <span className="text-sm text-accent">
            ✓ online{data.last_seen ? ' · last seen ' + compactAge(data.last_seen) + ' ago' : ''}
          </span>
        ) : (
          <span className="text-sm text-text-2">
            offline{data.last_seen ? ' · last ' + compactAge(data.last_seen) + ' ago' : ''}
          </span>
        )}
      </Field>
      <Field label="Equivocator">
        <span className={`text-sm ${data.is_equivocator ? 'text-bad' : 'text-text-1'}`}>
          {data.is_equivocator ? '⚠ yes — floored at -1.00' : 'no'}
        </span>
      </Field>
      {data.advertised_image_ref && (
        <Field label="Image">
          <code className="text-xs text-text-0 font-mono break-all">{data.advertised_image_ref}</code>
        </Field>
      )}
      {data.advertised_image_digest && (
        <Field label="Digest">
          <code className="text-xs text-text-0 font-mono">
            {shortenDigest(data.advertised_image_digest, 12)}
          </code>
        </Field>
      )}
      {data.advertised_capability && (
        <Field label="Capability">
          <Badge tone="violet" mono>{data.advertised_capability}</Badge>
        </Field>
      )}
      <Field label="Total entries">
        <span className="text-sm text-text-0">{data.entries_count}</span>
      </Field>
      <Field label="Verified raters">
        <span className="text-sm text-text-1">
          {data.rater_summary?.verified_raters_count ?? 0} verified ·{' '}
          {data.rater_summary?.unverified_raters_count ?? 0} unverified
        </span>
      </Field>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="text-[10px] uppercase tracking-wider text-text-2 mb-1">{label}</div>
      <div>{children}</div>
    </div>
  );
}
