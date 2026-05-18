import { useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { usePeer, usePeerLog } from '../lib/query';
import { useUIStore } from '../lib/store';
import { SummaryCard } from '../components/peer/SummaryCard';
import { Tabs } from '../components/peer/Tabs';
import { EntryRow } from '../components/peer/EntryRow';
import { Button } from '../components/primitives/Button';

type TabKind = 'all' | 'ratings' | 'comments';

export default function PeerView() {
  const { peerId } = useParams<{ peerId: string }>();
  const navigate = useNavigate();
  const openDispatch = useUIStore((s) => s.openDispatch);
  const [tab, setTab] = useState<TabKind>('all');

  const { data: summary, isPending: sPending, error: sErr } = usePeer(peerId);
  const { data: log, isPending: lPending, error: lErr } = usePeerLog(peerId, { limit: 100 });

  if (!peerId) {
    return <div className="p-7 text-rose-400">No peer id</div>;
  }
  if (sPending || lPending) {
    return <div className="p-7 text-text-2">Loading peer history…</div>;
  }
  if (sErr || lErr) {
    return <div className="p-7 text-rose-400">{(sErr || lErr)?.message}</div>;
  }
  if (!summary || !log) return <div className="p-7 text-text-2">No data.</div>;

  const allEntries = log.entries;
  const ratings = allEntries.filter((e) => e.kind === 'Rating');
  const comments = allEntries.filter((e) => e.kind === 'Comment');
  const view = tab === 'all' ? allEntries : tab === 'ratings' ? ratings : comments;

  return (
    <div className="p-7 max-w-5xl">
      <button
        onClick={() => navigate(-1)}
        className="inline-flex items-center gap-1.5 text-[11px] text-text-2 mb-3 hover:text-text-0"
      >
        ← back
      </button>

      <div className="flex justify-between items-start mb-2">
        <div>
          <h1 className="text-2xl font-semibold text-text-0">
            {summary.agent_name || '(unknown agent)'}
          </h1>
          <div className="font-mono text-[11px] text-text-2 mt-1 break-all">{summary.peer_id}</div>
        </div>
        <div className="flex gap-2">
          <Button onClick={() => navigate('/chat')}>Open in chat</Button>
          <Button
            variant="primary"
            disabled={!summary.dispatch_allowed}
            onClick={() => openDispatch(summary.peer_id)}
          >
            Dispatch task
          </Button>
        </div>
      </div>

      {summary.is_equivocator && (
        <div
          role="alert"
          className="my-4 border border-rose-500/50 bg-rose-500/10 rounded-lg p-4 flex gap-3"
        >
          <div className="text-rose-400 text-lg leading-none">⚠</div>
          <div>
            <div className="text-rose-300 font-semibold text-sm">Equivocator detected</div>
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

      <div className="my-4">
        <SummaryCard data={summary} />
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
            <EntryRow key={`${e.received_at}-${i}`} entry={e} peerId={summary.peer_id} />
          ))}
        </div>
      )}
    </div>
  );
}
