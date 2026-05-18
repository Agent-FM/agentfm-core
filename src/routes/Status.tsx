import { useEffect, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { useNavigate } from 'react-router-dom';
import { useAbout, useWorkers } from '../lib/query';
import { useBackend } from '../hooks/useBackend';
import { useUIStore } from '../lib/store';
import { LogsModal } from '../components/status/LogsModal';
import { Button } from '../components/primitives/Button';
import { toast } from 'sonner';

export default function Status() {
  const navigate = useNavigate();
  const { data: about } = useAbout();
  const { data: workers } = useWorkers(true);
  const backend = useBackend();
  const reputationFloor = useUIStore((s) => s.reputationFloor);

  const [showLogs, setShowLogs] = useState(false);
  const [showTech, setShowTech] = useState(false);

  const [uptimeSec, setUptimeSec] = useState(0);
  useEffect(() => {
    if (typeof about?.uptime_seconds === 'number') setUptimeSec(about.uptime_seconds);
  }, [about?.uptime_seconds]);
  useEffect(() => {
    const id = setInterval(() => setUptimeSec((s) => s + 1), 1000);
    return () => clearInterval(id);
  }, []);

  const relayConnected = !!about?.relay_peer_id;
  const workerCount = workers?.online_count ?? 0;
  const offlineCount = workers?.offline_count ?? 0;
  const equivocatorCount = workers?.agents.filter((a) => a.is_equivocator).length ?? 0;
  const ledgerSize = about?.ledger_tree_size ?? 0;

  const issues: string[] = [];
  if (!backend.ok) issues.push('Backend is offline');
  if (!relayConnected) issues.push('Not connected to a relay');
  if (equivocatorCount > 0) issues.push(`${equivocatorCount} equivocator${equivocatorCount > 1 ? 's' : ''} detected`);

  const allGood = issues.length === 0;

  async function copyPeerId() {
    if (!about?.boss_peer_id) return;
    try {
      await navigator.clipboard.writeText(about.boss_peer_id);
      toast.success('Your peer ID copied');
    } catch {
      toast.error('Could not copy');
    }
  }

  async function copyRelay() {
    if (!about?.relay_multiaddr) return;
    try {
      await navigator.clipboard.writeText(about.relay_multiaddr);
      toast.success('Relay multiaddr copied');
    } catch {
      toast.error('Could not copy');
    }
  }

  return (
    <>
      <div className="p-7 max-w-5xl">
        <h1 className="text-2xl font-semibold text-text-0">Your mesh</h1>
        <p className="text-text-2 mt-1 mb-5">
          A friendly view of what's happening right now. Technical details are tucked away below.
        </p>

        {/* Hero status banner */}
        <motion.div
          initial={{ opacity: 0, y: -6 }}
          animate={{ opacity: 1, y: 0 }}
          className={`rounded-2xl p-5 mb-6 border ${
            allGood
              ? 'bg-gradient-to-br from-accent-bg to-bg-1 border-accent/30'
              : 'bg-gradient-to-br from-rose-950/40 to-bg-1 border-rose-500/30'
          }`}
        >
          <div className="flex items-start gap-4">
            <div className="text-4xl leading-none">{allGood ? '✓' : '⚠'}</div>
            <div className="flex-1">
              <div className="text-lg font-semibold text-text-0">
                {allGood ? 'All systems are healthy' : `${issues.length} issue${issues.length > 1 ? 's' : ''} detected`}
              </div>
              <div className="text-sm text-text-1 mt-1">
                {allGood ? (
                  <>
                    Backend has been running for {formatUptime(uptimeSec)}. You're talking to{' '}
                    <span className="text-accent font-medium">{workerCount}</span>{' '}
                    online worker{workerCount === 1 ? '' : 's'}.
                  </>
                ) : (
                  <ul className="list-disc list-inside space-y-0.5">
                    {issues.map((i) => (
                      <li key={i}>{i}</li>
                    ))}
                  </ul>
                )}
              </div>
            </div>
            <div className="flex flex-col gap-2">
              <Button onClick={() => setShowLogs(true)}>View logs</Button>
              <Button
                variant="ghost"
                onClick={async () => {
                  try {
                    await window.api.backend.restart();
                    toast.success('Backend restarted');
                  } catch (e) {
                    toast.error('Restart failed: ' + (e as Error).message);
                  }
                }}
              >
                Restart backend
              </Button>
            </div>
          </div>
        </motion.div>

        {/* Friendly tiles */}
        <div className="grid grid-cols-3 gap-4">
          <FriendlyTile
            icon="🛰"
            label="Workers"
            value={workerCount}
            sub={
              <>
                online
                {offlineCount > 0 ? (
                  <span className="text-text-3"> · {offlineCount} known offline</span>
                ) : null}
              </>
            }
            cta={{ label: 'Open Radar', onClick: () => navigate('/radar') }}
          />
          <FriendlyTile
            icon="🔗"
            label="Relay"
            value={relayConnected ? 'Connected' : 'Not connected'}
            sub={
              relayConnected
                ? 'Your boss has reserved a circuit through the relay.'
                : 'Workers won\'t see you until you connect to a relay.'
            }
            cta={{
              label: relayConnected ? 'Copy multiaddr' : 'Configure relay',
              onClick: relayConnected ? copyRelay : () => navigate('/settings'),
            }}
            tone={relayConnected ? 'ok' : 'warn'}
          />
          <FriendlyTile
            icon="📜"
            label="Ledger entries"
            value={ledgerSize}
            sub="Ratings + comments signed by this boss."
            cta={{ label: 'See my activity', onClick: () => navigate('/activity') }}
          />
        </div>

        {/* Trust gate strip */}
        <div className="mt-5 bg-bg-1 border border-border-0 rounded-2xl p-5 flex items-center justify-between">
          <div>
            <div className="text-sm font-semibold text-text-0">Trust gate</div>
            <div className="text-text-2 text-sm mt-1">
              Workers with honesty below{' '}
              <span className="font-mono text-text-0">{reputationFloor.toFixed(2)}</span> are
              auto-refused. Equivocators are blocked permanently.
            </div>
          </div>
          <Button onClick={() => navigate('/settings')}>Adjust threshold</Button>
        </div>

        {/* Collapsible technical details */}
        <button
          onClick={() => setShowTech((v) => !v)}
          className="mt-7 text-text-2 hover:text-text-0 text-sm inline-flex items-center gap-1.5 transition-colors"
        >
          <span>{showTech ? '▾' : '▸'}</span>
          {showTech ? 'Hide technical details' : 'Show technical details'}
        </button>
        <AnimatePresence>
          {showTech && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              transition={{ duration: 0.18 }}
              className="overflow-hidden"
            >
              <div className="mt-3 bg-bg-1 border border-border-0 rounded-xl p-5 space-y-3">
                <TechRow k="Your peer ID" v={about?.boss_peer_id ?? '…'} onCopy={copyPeerId} />
                <TechRow k="Relay peer ID" v={about?.relay_peer_id || '(not connected)'} />
                <TechRow k="Relay multiaddr" v={about?.relay_multiaddr || '(none)'} onCopy={about?.relay_multiaddr ? copyRelay : undefined} />
                <TechRow k="Backend version" v={about?.version ?? '…'} />
                <TechRow k="Reputation floor" v={reputationFloor.toFixed(2)} />
                <TechRow k="Ledger storage" v="~/.agentfm/" />
                <TechRow k="API port" v={String(useBackendPort())} />
              </div>
            </motion.div>
          )}
        </AnimatePresence>
      </div>

      <LogsModal isOpen={showLogs} onClose={() => setShowLogs(false)} />
    </>
  );
}

function FriendlyTile({
  icon,
  label,
  value,
  sub,
  cta,
  tone = 'ok',
}: {
  icon: string;
  label: string;
  value: React.ReactNode;
  sub: React.ReactNode;
  cta: { label: string; onClick: () => void };
  tone?: 'ok' | 'warn';
}) {
  return (
    <motion.div
      whileHover={{ y: -2 }}
      transition={{ duration: 0.18 }}
      className="bg-bg-1 border border-border-0 rounded-2xl p-5 flex flex-col"
    >
      <div className="flex items-center gap-2 text-text-2 text-2xs uppercase tracking-wider">
        <span className="text-base">{icon}</span>
        {label}
      </div>
      <div
        className={`mt-2 text-2xl font-semibold ${
          tone === 'warn' ? 'text-amber-400' : 'text-text-0'
        }`}
      >
        {value}
      </div>
      <div className="text-text-2 text-sm mt-1 mb-4 flex-1">{sub}</div>
      <Button onClick={cta.onClick}>{cta.label}</Button>
    </motion.div>
  );
}

function TechRow({
  k,
  v,
  onCopy,
}: {
  k: string;
  v: string;
  onCopy?: () => void;
}) {
  return (
    <div className="flex items-baseline gap-3 text-sm">
      <div className="text-text-2 w-44 shrink-0">{k}</div>
      <div className="font-mono text-xs text-text-1 break-all flex-1">{v}</div>
      {onCopy && (
        <button
          onClick={onCopy}
          className="text-text-2 hover:text-accent text-xs shrink-0"
          title="Copy"
        >
          📋
        </button>
      )}
    </div>
  );
}

function useBackendPort(): number {
  return useUIStore((s) => s.apiPort);
}

function formatUptime(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  const m = Math.floor(seconds / 60);
  if (m < 60) return `${m} min`;
  const h = Math.floor(m / 60);
  const remM = m % 60;
  if (h < 24) return `${h}h ${remM}m`;
  const d = Math.floor(h / 24);
  return `${d}d ${h % 24}h`;
}
