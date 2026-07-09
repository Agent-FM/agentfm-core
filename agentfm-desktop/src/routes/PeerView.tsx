import { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { ArrowLeft, AlertOctagon } from 'lucide-react';
import { usePeer, usePeerLog } from '../lib/query';
import { useUIStore } from '../lib/store';
import { usePeerIdentityCache } from '../lib/peerIdentityCache';
import { motion } from 'framer-motion';
import { SummaryCard } from '../components/peer/SummaryCard';
import { TelemetryStrip } from '../components/peer/TelemetryStrip';
import { Tabs } from '../components/peer/Tabs';
import { EntryRow } from '../components/peer/EntryRow';
import { Button } from '../components/primitives/Button';
import { SkeletonBox, SkeletonRow } from '../components/primitives/Skeleton';
import { staggerItem } from '../lib/motion';
import { displayName } from '../lib/displayName';

type TabKind = 'all' | 'ratings' | 'comments';

export default function PeerView() {
  const { peerId } = useParams<{ peerId: string }>();
  const navigate = useNavigate();
  const openDispatch = useUIStore((s) => s.openDispatch);
  const [tab, setTab] = useState<TabKind>('all');

  const { data: summary, isPending: sPending, error: sErr } = usePeer(peerId);
  const { data: log, isPending: lPending, error: lErr } = usePeerLog(peerId, { limit: 100 });
  const cached = usePeerIdentityCache((s) => (peerId ? s.byPeerId[peerId] : undefined));

  if (!peerId) {
    return <div className="p-4 text-sm text-bad">No peer id</div>;
  }
  if (sPending || lPending) {
    return (
      <div className="p-4">
        <SkeletonBox className="h-3 w-24 mb-4" />
        <div className="flex justify-between items-start mb-4">
          <div className="min-w-0 flex-1 space-y-2">
            <SkeletonBox className="h-5 w-48" />
            <SkeletonBox className="h-2.5 w-72" />
          </div>
          <div className="flex gap-2">
            <SkeletonBox className="h-8 w-28" />
            <SkeletonBox className="h-8 w-28" />
          </div>
        </div>
        <SkeletonBox className="h-40 w-full mb-4" />
        <SkeletonBox className="h-24 w-full mb-4" />
        <div className="border-t border-border-0">
          {[0, 1, 2, 3].map((i) => (
            <SkeletonRow key={i} delay={i * 60} />
          ))}
        </div>
      </div>
    );
  }
  if (sErr || lErr) {
    return <div className="p-4 text-sm text-bad">{(sErr || lErr)?.message}</div>;
  }
  if (!summary || !log) return <div className="p-4 text-sm text-text-2">No data yet.</div>;

  const allEntries = log.entries ?? [];
  const ratings = allEntries.filter((e) => e.kind === 'Rating');
  const comments = allEntries.filter((e) => e.kind === 'Comment');
  const view = tab === 'all' ? allEntries : tab === 'ratings' ? ratings : comments;

  return (
    <div className="p-4">
      <button
        onClick={() => navigate('/radar')}
        className="inline-flex items-center gap-1.5 text-xs text-text-2 mb-2 hover:text-text-0 transition-colors duration-150"
      >
        <ArrowLeft size={14} strokeWidth={1.5} />
        <span>Back to Radar</span>
      </button>

      <div className="flex justify-between items-start mb-2">
        <div className="min-w-0 flex-1">
          <h1 className="text-lg font-semibold text-text-0">
            {displayName({ ...summary, name: summary.agent_name, peer_id: summary.peer_id }, cached)}
          </h1>
          <div className="font-mono text-2xs text-text-2 mt-1 break-all tabular-nums">
            {summary.peer_id}
            {(summary.author?.trim() || cached?.author) && (
              <>
                {' by '}
                <span className="text-accent font-semibold">
                  {summary.author?.trim() || cached?.author}
                </span>
              </>
            )}
          </div>
          {(summary.description?.trim() || cached?.description) && (
            <p className="text-sm text-text-1 mt-2 leading-relaxed whitespace-pre-line max-w-3xl">
              {summary.description?.trim() || cached?.description}
            </p>
          )}
        </div>
        <div className="flex gap-2">
          <Button
            disabled={!summary.online}
            title={!summary.online ? 'Agent is offline' : undefined}
            onClick={() => navigate('/chat')}
          >
            Open in chat
          </Button>
          <Button
            variant="primary"
            disabled={!summary.online || !summary.dispatch_allowed}
            title={!summary.online ? 'Agent is offline' : undefined}
            onClick={() => openDispatch(summary.peer_id)}
          >
            Dispatch task
          </Button>
        </div>
      </div>

      {summary.is_equivocator && (
        <div
          role="alert"
          className="mb-4 border border-bad/40 bg-bad/10 rounded-card p-3 flex gap-3"
        >
          <AlertOctagon size={16} strokeWidth={1.5} className="text-bad flex-none mt-0.5" />
          <div>
            <div className="text-bad font-semibold text-sm">Equivocator detected</div>
            <div className="text-xs text-text-1 mt-1">
              The mesh observed this peer publishing inconsistent telemetry or claims. Dispatch is
              auto-refused.
              {summary.dispatch_refuse_reason && (
                <>
                  {' '}Reason:{' '}
                  <span className="font-mono text-text-0">{summary.dispatch_refuse_reason}</span>
                </>
              )}
            </div>
            <div className="text-2xs text-text-2 mt-2">
              Equivocation is detected via cross-witness consistency proofs and is permanent,
              even if the agent later behaves correctly.
            </div>
          </div>
        </div>
      )}

      <div className="mb-4">
        <SummaryCard data={summary} />
      </div>

      <div className="mb-4">
        <TelemetryStrip peerId={summary.peer_id} />
      </div>

      <Tabs<TabKind>
        options={[
          { value: 'all', label: 'All', count: allEntries.length },
          { value: 'ratings', label: 'Ratings', count: ratings.length },
          { value: 'comments', label: 'Comments', count: comments.length },
        ]}
        value={tab}
        onChange={setTab}
      />

      {view.length === 0 ? (
        <div className="text-sm text-text-3 py-6 text-center">No entries match this filter.</div>
      ) : (
        <div className="border-t border-border-0">
          {view.map((e, i) => (
            <motion.div
              key={`${e.received_at}-${i}`}
              className="border-b border-border-0 hover:bg-white/[0.04] transition-colors duration-150"
              {...staggerItem(Math.min(i, 12))}
            >
              <EntryRow entry={e} peerId={summary.peer_id} />
            </motion.div>
          ))}
        </div>
      )}
    </div>
  );
}
