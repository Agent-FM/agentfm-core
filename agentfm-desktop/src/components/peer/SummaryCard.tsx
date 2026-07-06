import type { PeerSummary } from '../../types/api';
import { compactAge } from '../../lib/peer';
import { Badge } from '../primitives/Badge';
import { Card } from '../primitives/Card';
import { StarRow } from '../primitives/StarRow';
import { starsFromScore } from '../../lib/stars';
import { Copy } from 'lucide-react';
import { toast } from 'sonner';

export function SummaryCard({ data }: { data: PeerSummary }) {
  const score = data.honesty_score;
  return (
    <Card className="grid grid-cols-3 gap-x-6 gap-y-4">
      <Field label="Rating">
        <div className="flex gap-1.5 flex-wrap items-center">
          {data.is_equivocator ? (
            <Badge tone="rose">⚠ equivocator</Badge>
          ) : (
            <>
              <StarRow value={starsFromScore(score)} size={15} />
              <Badge tone={score > 0.3 ? 'lime' : score < -0.5 ? 'rose' : 'neutral'} mono>
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
          <span className="text-sm text-accent tabular-nums">
            ✓ online{data.last_seen ? ' · last seen ' + compactAge(data.last_seen) + ' ago' : ''}
          </span>
        ) : (
          <span className="text-sm text-text-2 tabular-nums">
            offline{data.last_seen ? ' · last ' + compactAge(data.last_seen) + ' ago' : ''}
          </span>
        )}
      </Field>
      <Field label="Equivocator">
        <span className={`text-sm tabular-nums ${data.is_equivocator ? 'text-bad' : 'text-text-1'}`}>
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
          <button
            onClick={() =>
              navigator.clipboard
                .writeText(data.advertised_image_digest as string)
                .then(() => toast.success('Digest copied'))
                .catch(() => toast.error('Copy failed'))
            }
            title={`${data.advertised_image_digest} — click to copy`}
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
          <Badge tone="cyan" mono>{data.advertised_capability}</Badge>
        </Field>
      )}
      <Field label="Total entries">
        <span className="text-sm text-text-0 tabular-nums">{data.entries_count}</span>
      </Field>
      <Field label="Verified raters">
        <span className="text-sm text-text-1 tabular-nums">
          {data.rater_summary?.verified_raters_count ?? 0} verified ·{' '}
          {data.rater_summary?.unverified_raters_count ?? 0} unverified
        </span>
      </Field>
    </Card>
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
