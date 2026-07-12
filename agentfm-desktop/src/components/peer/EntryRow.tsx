import { Copy } from 'lucide-react';
import { toast } from 'sonner';
import type { PeerEntry } from '../../types/api';
import { compactAge, shortenPeerID } from '../../lib/peer';
import { CommentBody } from './CommentBody';
import { StarRow } from '../primitives/StarRow';
import { starsFromScore } from '../../lib/stars';

interface Props {
  entry: PeerEntry;
  peerId: string;
}

export function EntryRow({ entry, peerId }: Props) {
  const isComment = entry.kind === 'Comment';

  return (
    <div>
      <div className="grid grid-cols-[80px_84px_1fr_70px] gap-3 px-2 py-1 min-h-6 items-start text-sm">
        <div
          className={`text-2xs font-medium pt-0.5 ${
            entry.kind === 'Rating' ? 'text-accent' : 'text-text-1'
          }`}
        >
          {entry.kind}
        </div>
        <div className="font-mono text-xs tabular-nums pt-0.5">
          {typeof entry.score === 'number' ? (
            <div className="flex flex-col gap-0.5">
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
              <StarRow value={starsFromScore(entry.score)} size={10} />
            </div>
          ) : null}
        </div>
        <div className="min-w-0">
          <div className="flex gap-1.5 items-baseline">
            {entry.rater_peer_id && (
              <button
                onClick={() =>
                  navigator.clipboard
                    .writeText(entry.rater_peer_id)
                    .then(() => toast.success('Rater peer ID copied'))
                    .catch(() => toast.error('Copy failed'))
                }
                title={`Copy rater peer ID ${entry.rater_peer_id}`}
                className="group/rid inline-flex items-center gap-1 font-mono text-2xs text-text-2 hover:text-text-0 transition-colors shrink-0 cursor-pointer"
              >
                {shortenPeerID(entry.rater_peer_id, 3, 3)}
                <Copy size={10} className="opacity-0 group-hover/rid:opacity-70 transition-opacity" />
              </button>
            )}
            {entry.context && <span className="text-text-2 text-xs">{entry.context}</span>}
          </div>
          {isComment && entry.text_cid && <CommentBody peerId={peerId} cid={entry.text_cid} />}
        </div>
        <div className="text-2xs text-text-2 text-right font-mono tabular-nums pt-0.5">
          {compactAge(entry.received_at)}
        </div>
      </div>
    </div>
  );
}
