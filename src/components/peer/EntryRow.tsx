import { useState } from 'react';
import type { PeerEntry } from '../../types/api';
import { compactAge, shortenPeerID } from '../../lib/peer';
import { CommentBodyExpander } from './CommentBody';

interface Props {
  entry: PeerEntry;
  peerId: string;
}

export function EntryRow({ entry, peerId }: Props) {
  const [expanded, setExpanded] = useState(false);
  const isComment = entry.kind === 'Comment';
  const unverified = entry.rater_status === 'unverified';

  return (
    <div className="grid grid-cols-[80px_70px_1fr_70px] gap-3.5 py-2.5 border-b border-border-0/40 items-center text-sm">
      <div className="text-[11px] uppercase tracking-wider text-text-2">{entry.kind}</div>
      <div className="font-mono font-medium">
        {typeof entry.score === 'number' ? (
          <span
            className={
              entry.score > 0
                ? 'text-emerald-400'
                : entry.score < 0
                  ? 'text-rose-400'
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
      <div>
        <div className="flex gap-1.5 items-baseline">
          {unverified && (
            <span className="text-[10px] bg-bg-2 text-text-2 px-1.5 py-0.5 rounded">
              unverified
            </span>
          )}
          <span className="font-mono text-[11px] text-text-1">
            {shortenPeerID(entry.rater_peer_id, 6, 5)}
          </span>
          {entry.context && <span className="text-text-2 text-xs">{entry.context}</span>}
          {entry.language && <span className="text-text-2 text-xs">[{entry.language}]</span>}
        </div>
        {isComment && entry.text_cid && (
          <CommentBodyExpander
            peerId={peerId}
            cid={entry.text_cid}
            expanded={expanded}
            onToggle={() => setExpanded(!expanded)}
          />
        )}
      </div>
      <div className="text-[11px] text-text-2 text-right font-mono">
        {compactAge(entry.received_at)}
      </div>
    </div>
  );
}
