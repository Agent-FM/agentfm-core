import { useMemo } from 'react';
import { Star, MessageSquare } from 'lucide-react';
import { useQueries } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { useAbout, useWorkers, qk } from '../lib/query';
import { api } from '../lib/api';
import { EntryRow } from '../components/peer/EntryRow';
import type { PeerEntry } from '../types/api';
import { usePeerName } from '../hooks/usePeerName';
import { shortenPeerID } from '../lib/peer';

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

  // Fan out per peer, N+1 at hobbyist scale (<10 peers)
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
    <div className="p-4">
      <h1 className="text-lg font-semibold text-text-0">My activity</h1>
      <p className="text-sm text-text-2 mt-1 mb-4">
        Every rating and comment you have signed and broadcast to the mesh.
        {someLoading && (
          <span className="ml-2 tabular-nums text-text-3">
            Refreshing across {peers.length} peers
          </span>
        )}
      </p>

      {myEntries.length === 0 && !someLoading ? (
        <div className="py-16 text-center">
          <p className="text-base font-semibold text-text-1">No outgoing entries yet</p>
          <p className="text-sm text-text-3 mt-1.5">
            Leave feedback on a peer from their profile or run a dispatch. Entries you sign show up
            here.
          </p>
        </div>
      ) : (
        <div className="space-y-5">
          {orderedBuckets.map((b) => (
            <section key={b}>
              <div className="mb-2 flex items-center gap-2">
                <span className="text-2xs font-medium text-text-2">{BUCKET_LABEL[b]}</span>
                <span className="text-2xs text-text-3 tabular-nums">{grouped[b].length}</span>
              </div>
              <div className="border border-border-0 rounded-card overflow-hidden">
                {grouped[b].map(({ subject, entry }, i) => (
                  <div
                    key={`${entry.received_at}-${i}`}
                    className="flex gap-3 px-3 py-2 border-b border-border-0 last:border-b-0 hover:bg-white/[0.04] transition-colors duration-150"
                  >
                    <span
                      className={`mt-0.5 shrink-0 w-6 h-6 rounded-ctl flex items-center justify-center ${
                        entry.kind === 'Comment'
                          ? 'bg-white/[0.06] text-text-1'
                          : 'bg-accent/15 text-accent'
                      }`}
                      aria-hidden="true"
                    >
                      {entry.kind === 'Comment'
                        ? <MessageSquare size={13} strokeWidth={1.5} />
                        : <Star size={13} strokeWidth={1.5} />}
                    </span>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-baseline gap-1.5 text-sm min-w-0">
                        <span className="text-text-1 shrink-0">
                          You {entry.kind === 'Comment' ? 'commented on' : 'rated'}
                        </span>
                        <button
                          onClick={() => navigate(`/peer/${subject}`)}
                          className="font-medium text-text-0 hover:text-accent transition-colors truncate cursor-pointer"
                          title={`${subject}, open profile`}
                        >
                          <PeerName peerId={subject} />
                        </button>
                        <span className="font-mono text-2xs text-text-2 shrink-0">
                          {shortenPeerID(subject, 6, 4)}
                        </span>
                      </div>
                      <EntryRow entry={entry} peerId={subject} />
                    </div>
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
