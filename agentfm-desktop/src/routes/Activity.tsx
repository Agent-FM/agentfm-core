import { useMemo } from 'react';
import { useQueries } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { useAbout, useWorkers, qk } from '../lib/query';
import { api } from '../lib/api';
import { EntryRow } from '../components/peer/EntryRow';
import type { PeerEntry } from '../types/api';
import { usePeerName } from '../hooks/usePeerName';
import { StatusDot } from '../components/primitives/StatusDot';

interface ActivityEntry {
  subject: string;
  entry: PeerEntry;
}

type Bucket = 'today' | 'yesterday' | 'older';

function bucketFor(date: Date, now: Date): Bucket {
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  const t = date.getTime();
  if (t >= startOfToday) return 'today';
  if (t >= startOfToday - 24 * 3600 * 1000) return 'yesterday';
  return 'older';
}

const BUCKET_LABEL: Record<Bucket, string> = {
  today: 'Today',
  yesterday: 'Yesterday',
  older: 'Older',
};

export default function Activity() {
  const navigate = useNavigate();
  const { data: about } = useAbout();
  const { data: workers } = useWorkers(true);
  const me = about?.boss_peer_id;
  const peers = workers?.agents.map((w) => w.peer_id) ?? [];

  // Fan out per peer — N+1 at hobbyist scale (<10 peers)
  const queries = useQueries({
    queries: peers.map((pid) => ({
      queryKey: qk.peerLog(pid, { limit: 200, offset: 0 }),
      queryFn: () => api.peerLog(pid, { limit: 200, offset: 0 }),
      enabled: !!me,
      staleTime: 10000,
    })),
  });

  const myEntries = useMemo<ActivityEntry[]>(() => {
    if (!me) return [];
    const flat: ActivityEntry[] = [];
    for (let i = 0; i < queries.length; i++) {
      const q = queries[i];
      const subject = peers[i];
      if (q.data?.entries) {
        for (const e of q.data.entries) {
          if (e.rater_peer_id === me) flat.push({ subject, entry: e });
        }
      }
    }
    return flat.sort(
      (a, b) =>
        new Date(b.entry.received_at).getTime() - new Date(a.entry.received_at).getTime(),
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [queries, me, peers.join(',')]);

  const grouped = useMemo(() => {
    const now = new Date();
    const buckets: Record<Bucket, ActivityEntry[]> = { today: [], yesterday: [], older: [] };
    for (const it of myEntries) {
      const b = bucketFor(new Date(it.entry.received_at), now);
      buckets[b].push(it);
    }
    return buckets;
  }, [myEntries]);

  const someLoading = queries.some((q) => q.isPending);
  const orderedBuckets: Bucket[] = (['today', 'yesterday', 'older'] as Bucket[]).filter(
    (b) => grouped[b].length > 0,
  );

  return (
    <div className="p-7 max-w-5xl">
      <h1 className="text-2xl font-semibold tracking-tight text-text-0">My activity</h1>
      <p className="text-sm text-text-2 mt-1 mb-5">
        Every rating and comment <em>you</em> have signed and broadcast to the mesh.
        {someLoading && (
          <span className="ml-2">refreshing across {peers.length} peers…</span>
        )}
      </p>

      {myEntries.length === 0 && !someLoading ? (
        <div className="bg-bg-1 border border-border-0 rounded-lg p-8 text-center">
          <div className="text-4xl mb-3 opacity-50">📜</div>
          <p className="text-text-1 font-medium">No outgoing entries yet.</p>
          <p className="text-sm text-text-2 mt-2">
            Leave feedback on a peer from their profile, or run a dispatch — entries you sign show
            up here.
          </p>
        </div>
      ) : (
        <div className="space-y-5">
          {orderedBuckets.map((b) => (
            <section key={b}>
              <h2 className="text-2xs uppercase tracking-wider text-text-2 mb-2 px-1 inline-flex items-center">
                <StatusDot tone="cyan" size="sm" className="mr-2" />
                {BUCKET_LABEL[b]}{' '}
                <span className="text-text-3 normal-case ml-1">({grouped[b].length})</span>
              </h2>
              <div className="bg-bg-1 border border-border-0 rounded-lg p-3">
                {grouped[b].map(({ subject, entry }, i) => (
                  <div
                    key={`${entry.received_at}-${i}`}
                    className="last:border-0"
                    style={{ borderBottom: '1px solid rgba(34,211,238,.08)' }}
                  >
                    <button
                      onClick={() => navigate(`/peer/${subject}`)}
                      className="block w-full text-left text-2xs text-text-2 px-1 pt-2.5 hover:text-accent"
                    >
                      about <span className="text-accent2-light hover:glow-text-violet transition-colors font-medium"><PeerName peerId={subject} /></span>
                      <span className="ml-2 font-mono text-text-3">{subject.slice(0, 12)}…</span>
                    </button>
                    <EntryRow entry={entry} peerId={subject} />
                  </div>
                ))}
              </div>
            </section>
          ))}
        </div>
      )}
    </div>
  );
}

function PeerName({ peerId }: { peerId: string }) {
  return <>{usePeerName(peerId)}</>;
}
