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
    return <div className="p-6 text-bad">No peer id</div>;
  }
  if (sPending || lPending) {
    return <div className="p-6 text-text-2">Loading peer history…</div>;
  }
  if (sErr || lErr) {
    return <div className="p-6 text-bad">{(sErr || lErr)?.message}</div>;
  }
  if (!summary || !log) return <div className="p-6 text-text-2">No data.</div>;

  const allEntries = log.entries ?? [];
  const ratings = allEntries.filter((e) => e.kind === 'Rating');
  const comments = allEntries.filter((e) => e.kind === 'Comment');
  const view = tab === 'all' ? allEntries : tab === 'ratings' ? ratings : comments;

  return (
    <div className="p-6 max-w-5xl">
      <button
        onClick={() => navigate('/radar')}
        className="inline-flex items-center gap-1.5 text-xs text-text-2 mb-3 hover:text-text-0"
      >
        <ArrowLeft size={14} />
        <span>Back to Radar</span>
      </button>

      <div className="flex justify-between items-start mb-2">
        <div className="min-w-0 flex-1">
          <h1 className="text-3xl font-semibold tracking-tight text-text-0">
            {displayName({ ...summary, name: summary.agent_name, peer_id: summary.peer_id }, cached)}
          </h1>
          <div className="font-mono text-[11px] text-text-2 mt-1 break-all tabular-nums">
            {summary.peer_id}
            {(summary.author?.trim() || cached?.author) && (
              <>
                {' · by '}
                <span className="text-accent font-semibold">
                  {summary.author?.trim() || cached?.author}
                </span>
              </>
            )}
          </div>
          {(summary.description?.trim() || cached?.description) && (
            <p className="text-[14px] text-text-1 mt-3 leading-[1.55] whitespace-pre-line max-w-3xl">
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
          className="mb-6 border border-bad/40 bg-bad/10 rounded-[14px] p-5 flex gap-3"
        >
          <AlertOctagon size={24} className="text-bad flex-none mt-0.5" />
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
            <div className="text-[11px] text-text-2 mt-2">
              Equivocation is detected via cross-witness consistency proofs and is permanent —
              even if the agent later behaves correctly.
            </div>
          </div>
        </div>
      )}

      <div className="mb-6">
        <SummaryCard data={summary} />
      </div>

      <div className="mb-6">
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
        <div className="text-sm text-text-2 py-6">No entries match this filter.</div>
      ) : (
        <div>
          {view.map((e, i) => (
            <motion.div key={`${e.received_at}-${i}`} {...staggerItem(Math.min(i, 12))}>
              <EntryRow entry={e} peerId={summary.peer_id} />
            </motion.div>
          ))}
        </div>
      )}
    </div>
  );
}
