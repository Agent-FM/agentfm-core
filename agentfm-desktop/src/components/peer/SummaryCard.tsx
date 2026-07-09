import type { PeerSummary } from '../../types/api';
import { Badge } from '../primitives/Badge';
import { Card } from '../primitives/Card';
import { StarRow } from '../primitives/StarRow';
import { starsFromScore } from '../../lib/stars';
import { Copy, AlertTriangle } from 'lucide-react';
import { toast } from 'sonner';

export function SummaryCard({ data }: { data: PeerSummary }) {
  const score = data.honesty_score;
  return (
    <Card density="compact" className="space-y-0.5">
      <Field label="Rating">
        <div className="flex gap-1.5 flex-wrap items-center">
          {data.is_equivocator ? (
            <Badge tone="bad"><AlertTriangle size={11} strokeWidth={1.5} /> equivocator</Badge>
          ) : (
            <>
              <StarRow value={starsFromScore(score)} size={15} />
              <Badge tone={score > 0.3 ? 'ok' : score < -0.5 ? 'bad' : 'neutral'} mono>
                <span className="tabular-nums">
                  {score >= 0 ? '+' : ''}{score.toFixed(2)}
                </span>{' '}
                {score > 0.3 ? 'honest' : score < -0.5 ? 'flagged' : 'neutral'}
              </Badge>
            </>
          )}
        </div>
      </Field>
      <Field label="Status">
        {data.online ? (
          <span className="inline-flex items-center gap-1.5 text-sm text-text-0 tabular-nums">
            <span className="inline-block w-1.5 h-1.5 rounded-full bg-ok" aria-hidden="true" />
            online
          </span>
        ) : (
          <span className="text-sm text-text-2 tabular-nums">offline</span>
        )}
      </Field>
      <Field label="Equivocator">
        <span className={`inline-flex items-center gap-1 text-sm tabular-nums ${data.is_equivocator ? 'text-bad' : 'text-text-1'}`}>
          {data.is_equivocator ? (
            <>
              <AlertTriangle size={12} strokeWidth={1.5} className="flex-none" />
              yes, floored at -1.00
            </>
          ) : (
            'no'
          )}
        </span>
      </Field>
      {data.advertised_image_ref && (
        <Field label="Image">
          <code className="text-xs text-text-0 font-mono break-all">{data.advertised_image_ref}</code>
        </Field>
      )}
      {data.advertised_image_digest && (
        <Field label="Digest">
          <button
            onClick={() =>
              navigator.clipboard
                .writeText(data.advertised_image_digest as string)
                .then(() => toast.success('Digest copied'))
                .catch(() => toast.error('Copy failed'))
            }
            title={`${data.advertised_image_digest}, click to copy`}
            className="group/dg inline-flex items-start gap-1 text-left"
          >
            <code className="text-xs text-text-0 font-mono break-all">
              {data.advertised_image_digest}
            </code>
            <Copy
              size={11}
              className="mt-0.5 shrink-0 opacity-0 group-hover/dg:opacity-60 transition-opacity"
            />
          </button>
        </Field>
      )}
      {data.advertised_capability && (
        <Field label="Capability">
          <Badge tone="accent" mono>{data.advertised_capability}</Badge>
        </Field>
      )}
      <Field label="Total entries">
        <span className="font-mono text-xs text-text-0 tabular-nums">{data.entries_count}</span>
      </Field>
      <Field label="Verified raters">
        <span className="inline-flex items-center gap-2 font-mono text-xs text-text-1 tabular-nums">
          <span>{data.rater_summary?.verified_raters_count ?? 0} verified</span>
          <span className="w-px h-3 bg-border-1" aria-hidden="true" />
          <span>{data.rater_summary?.unverified_raters_count ?? 0} unverified</span>
        </span>
      </Field>
    </Card>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid grid-cols-[130px_1fr] gap-3 items-baseline min-h-6 py-0.5">
      <div className="text-sm text-text-1">{label}</div>
      <div className="text-sm min-w-0">{children}</div>
    </div>
  );
}
