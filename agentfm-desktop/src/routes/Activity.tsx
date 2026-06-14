import { useMemo } from 'react';
import { motion } from 'framer-motion';
import { useQueries } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { useAbout, useWorkers, qk } from '../lib/query';
import { api } from '../lib/api';
import { EntryRow } from '../components/peer/EntryRow';
import type { PeerEntry } from '../types/api';
import { usePeerName } from '../hooks/usePeerName';
import { Card } from '../components/primitives/Card';
import { SectionLabel } from '../components/primitives/SectionLabel';
import { staggerItem } from '../lib/motion';

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
    <div className="p-6 max-w-5xl">
      <h1 className="text-2xl font-semibold tracking-tight text-text-0">My activity</h1>
      <p className="text-sm text-text-2 mt-1 mb-6">
        Every rating and comment <em>you</em> have signed and broadcast to the mesh.
        {someLoading && (
          <span className="ml-2 tabular-nums">refreshing across {peers.length} peers…</span>
        )}
      </p>

      {myEntries.length === 0 && !someLoading ? (
        <div className="bg-bg-2 border border-border-0 rounded-lg p-8 text-center">
          <p className="text-text-1 font-medium">No outgoing entries yet.</p>
          <p className="text-sm text-text-2 mt-2">
            Leave feedback on a peer from their profile, or run a dispatch — entries you sign show
            up here.
          </p>
        </div>
      ) : (
        <div className="space-y-6">
          {(() => {
            let rowIndex = 0;
            return orderedBuckets.map((b) => (
              <section key={b}>
                <div className="mb-2 px-1 flex items-center gap-2">
                  <SectionLabel tone="cyan">{BUCKET_LABEL[b]}</SectionLabel>
                  <span className="text-2xs text-text-3 tabular-nums">({grouped[b].length})</span>
                </div>
                <Card density="compact" className="divide-y divide-border-0">
                  {grouped[b].map(({ subject, entry }, i) => (
                    <motion.div
                      key={`${entry.received_at}-${i}`}
                      className="px-4 py-3"
                      {...staggerItem(Math.min(rowIndex++, 12))}
                    >
                      <button
                        onClick={() => navigate(`/peer/${subject}`)}
                        className="block w-full text-left text-2xs text-text-2 hover:text-accent transition-colors"
                      >
                        about{' '}
                        <span className="text-text-1 font-medium">
                          <PeerName peerId={subject} />
                        </span>
                        <span className="ml-2 font-mono text-text-3 tabular-nums">
                          {subject.slice(0, 12)}…
                        </span>
                      </button>
                      <EntryRow entry={entry} peerId={subject} />
                    </motion.div>
                  ))}
                </Card>
              </section>
            ));
          })()}
        </div>
      )}
    </div>
  );
}

function PeerName({ peerId }: { peerId: string }) {
  return <>{usePeerName(peerId)}</>;
}
