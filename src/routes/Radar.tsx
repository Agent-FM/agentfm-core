import { useMemo, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { useNavigate } from 'react-router-dom';
import { useWorkers } from '../lib/query';
import { useUIStore } from '../lib/store';
import { AgentCard } from '../components/AgentCard';
import { Input } from '../components/primitives/Input';
import { EmptyRadar } from '../components/EmptyRadar';
import { RadarSkeleton } from '../components/RadarSkeleton';
import type { WorkerProfile } from '../types/api';

type FilterPill = 'all' | 'trusted' | 'available' | 'capability';

export default function Radar() {
  const { data, isPending, error, refetch } = useWorkers(true);
  const navigate = useNavigate();
  const openDispatch = useUIStore((s) => s.openDispatch);
  const search = useUIStore((s) => s.searchTerm);
  const setSearch = useUIStore((s) => s.setSearchTerm);
  const [activeFilter, setActiveFilter] = useState<FilterPill>('all');
  const [capabilityFilter, setCapabilityFilter] = useState<string | null>(null);

  const agents = data?.agents ?? [];
  const allCapabilities = useMemo(() => {
    const set = new Set<string>();
    agents.forEach((a) => a.agent_capability && set.add(a.agent_capability));
    return Array.from(set).sort();
  }, [agents]);

  const filtered = useMemo(() => {
    return agents.filter((a) => {
      const matchesSearch =
        !search ||
        [a.name, a.peer_id, a.agent_image_ref, a.agent_image_digest, a.agent_capability].some(
          (f) => f && f.toLowerCase().includes(search.toLowerCase()),
        );
      if (!matchesSearch) return false;

      switch (activeFilter) {
        case 'all':
          return true;
        case 'trusted':
          return a.honesty_score > 0.3 && !a.is_equivocator;
        case 'available':
          return (
            a.online && a.dispatch_allowed && a.current_tasks < a.max_tasks
          );
        case 'capability':
          return !capabilityFilter || a.agent_capability === capabilityFilter;
      }
    });
  }, [agents, search, activeFilter, capabilityFilter]);

  const online = filtered.filter((a) => a.online);
  const offline = filtered.filter((a) => !a.online);

  if (isPending) {
    return <RadarSkeleton />;
  }
  if (error) {
    return (
      <div className="p-7">
        <div className="text-rose-400 mb-3">{(error as Error).message}</div>
        <button
          onClick={() => refetch()}
          className="text-xs bg-bg-2 border border-border-0 rounded-md px-3 py-1.5"
        >
          Retry
        </button>
      </div>
    );
  }

  // Friendly empty state when the mesh has discovered zero agents
  // (i.e. no online + no offline cached). Only when search/filter is unset
  // so we don't hide it behind a stale filter the user forgot to clear.
  if (agents.length === 0 && !search && activeFilter === 'all') {
    return (
      <div className="p-7">
        <div className="flex items-baseline gap-3 mb-1">
          <h1 className="text-xl font-semibold text-text-0">Agent Radar</h1>
          <span className="inline-flex gap-1.5 items-center bg-accent-bg text-accent text-[10px] uppercase tracking-wider px-2 py-0.5 rounded-full font-semibold">
            <span className="w-1 h-1 rounded-full bg-accent" />
            LIVE
          </span>
        </div>
        <p className="text-sm text-text-2 mb-5">
          Listening for workers announcing on the gossip topic.
        </p>
        <EmptyRadar />
      </div>
    );
  }

  return (
    <div className="p-7">
      <div className="flex items-baseline gap-3 mb-1">
        <h1 className="text-xl font-semibold text-text-0">Agent Radar</h1>
        <span className="inline-flex gap-1.5 items-center bg-accent-bg text-accent text-[10px] uppercase tracking-wider px-2 py-0.5 rounded-full font-semibold">
          <span className="w-1 h-1 rounded-full bg-accent" />
          LIVE
        </span>
      </div>
      <p className="text-sm text-text-2 mb-5">
        Every worker the mesh has heard of. Online updates in real time; offline is from the local
        archive.
      </p>

      <div className="flex justify-between items-center mb-4">
        <div className="relative">
          <span className="absolute left-2.5 top-1/2 -translate-y-1/2 text-xs">🔍</span>
          <Input
            placeholder="Search by name, image, or peer id…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-7 w-72"
          />
        </div>
        <div className="flex gap-1.5 flex-wrap">
          {(['all', 'trusted', 'available', 'capability'] as FilterPill[]).map((f) => (
            <button
              key={f}
              onClick={() => setActiveFilter(f)}
              className={`text-[11px] px-2.5 py-1 rounded-full border transition-colors ${
                activeFilter === f
                  ? 'bg-accent-bg border-accent/30 text-accent'
                  : 'bg-bg-2 border-border-0 text-text-2 hover:text-text-0'
              }`}
            >
              {f === 'all'
                ? 'All'
                : f === 'trusted'
                  ? 'Trusted'
                  : f === 'available'
                    ? 'Available'
                    : 'Capability'}
            </button>
          ))}
          {activeFilter === 'capability' && allCapabilities.length > 0 && (
            <select
              value={capabilityFilter ?? ''}
              onChange={(e) => setCapabilityFilter(e.target.value || null)}
              className="text-[11px] bg-bg-2 border border-border-0 rounded-md px-2 py-1 text-text-1"
            >
              <option value="">All capabilities</option>
              {allCapabilities.map((c) => (
                <option key={c} value={c}>
                  {c}
                </option>
              ))}
            </select>
          )}
        </div>
      </div>

      <Section title="Online" count={online.length}>
        <AnimatePresence initial={false} mode="popLayout">
          {online.map((w) => (
            <motion.div
              key={w.peer_id}
              layout
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -8 }}
              transition={{ duration: 0.2 }}
            >
              <AgentCard
                worker={w}
                onHistory={() => navigate(`/peer/${w.peer_id}`)}
                onDispatch={() => openDispatch(w.peer_id)}
              />
            </motion.div>
          ))}
        </AnimatePresence>
        {online.length === 0 && (
          <div className="text-sm text-text-2 py-3">No online agents match your filter.</div>
        )}
      </Section>

      <Section title="Offline" count={offline.length} className="mt-7">
        <AnimatePresence initial={false} mode="popLayout">
          {offline.map((w) => (
            <motion.div
              key={w.peer_id}
              layout
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -8 }}
              transition={{ duration: 0.2 }}
            >
              <AgentCard
                worker={w}
                onHistory={() => navigate(`/peer/${w.peer_id}`)}
                onDispatch={() => openDispatch(w.peer_id)}
              />
            </motion.div>
          ))}
        </AnimatePresence>
        {offline.length === 0 && (
          <div className="text-sm text-text-2 py-3">No offline agents.</div>
        )}
      </Section>
    </div>
  );
}

function Section({
  title,
  count,
  children,
  className = '',
}: {
  title: string;
  count: number;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={className}>
      <div className="text-[11px] uppercase tracking-wider text-text-2 mb-2.5 flex items-center gap-2">
        {title}
        <span className="text-[10px] bg-bg-2 text-text-3 px-2 py-0.5 rounded-full">{count}</span>
      </div>
      {children}
    </div>
  );
}
