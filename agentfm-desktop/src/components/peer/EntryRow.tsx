import type { PeerEntry } from '../../types/api';
import { compactAge } from '../../lib/peer';
import { CommentBody } from './CommentBody';

interface Props {
  entry: PeerEntry;
  peerId: string;
}

export function EntryRow({ entry, peerId }: Props) {
  const isComment = entry.kind === 'Comment';
  const unverified = entry.rater_status === 'unverified';
  const hasMeta = unverified || !!entry.context;

  return (
    <div className="relative pl-4 border-b border-border-0/40">
      <span
        className={`absolute left-0 top-2 bottom-2 w-[3px] rounded-full ${
          entry.kind === 'Rating' ? 'bg-accent' : 'bg-border-1'
        }`}
      />
      <div className="grid grid-cols-[80px_70px_1fr_70px] gap-3.5 py-2.5 items-start text-sm">
        <div className="text-[11px] uppercase tracking-wider text-text-2 pt-0.5">{entry.kind}</div>
        <div className="font-mono font-medium tabular-nums pt-0.5">
          {typeof entry.score === 'number' ? (
            <span
              className={
                entry.score > 0
                  ? 'text-accent'
                  : entry.score < 0
                    ? 'text-bad'
                    : 'text-text-1'
              }
            >
              {entry.score > 0 ? '+' : ''}
              {entry.score.toFixed(2)}
            </span>
          ) : (
            <span className="text-text-3">—</span>
          )}
        </div>
        <div className="min-w-0">
          {hasMeta && (
            <div className="flex gap-1.5 items-baseline">
              {unverified && (
                <span className="text-[10px] bg-bg-2 text-text-2 px-1.5 py-0.5 rounded">
                  unverified
                </span>
              )}
              {entry.context && <span className="text-text-2 text-xs">{entry.context}</span>}
            </div>
          )}
          {isComment && entry.text_cid && <CommentBody peerId={peerId} cid={entry.text_cid} />}
        </div>
        <div className="text-[11px] text-text-2 text-right font-mono tabular-nums pt-0.5">
          {compactAge(entry.received_at)}
        </div>
      </div>
    </div>
  );
}
