import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { FileText, RefreshCw, Radar as RadarIcon, ScrollText } from 'lucide-react'
import { useAbout } from '../lib/query'
import { useBackend } from '../hooks/useBackend'
import { useUIStore } from '../lib/store'
import { SectionLabel } from '../components/primitives/SectionLabel'
import { HeroTitle } from '../components/primitives/HeroTitle'
import { NeonCard } from '../components/primitives/NeonCard'
import { Avatar } from '../components/primitives/Avatar'
import { GradientButton } from '../components/primitives/GradientButton'
import { shortenPeerID } from '../lib/peer'
import { LogsModal } from '../components/status/LogsModal'

function formatUptime(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = seconds % 60
  return [h, m, s].map((n) => String(n).padStart(2, '0')).join(':')
}

interface StatCardProps {
  label: string
  emoji: string
  value: string
  valueTone: 'ok' | 'cyan' | 'violet' | 'rose'
  sub: string
}

function StatCard({ label, emoji, value, valueTone, sub }: StatCardProps) {
  const valueColor = {
    ok: '#84cc16', cyan: '#22d3ee', violet: '#a855f7', rose: '#f43f5e',
  }[valueTone]
  return (
    <NeonCard breathing className="p-4">
      <div className="flex items-center justify-between mb-3">
        <div className="font-mono uppercase font-bold text-text-2"
             style={{fontSize:10,letterSpacing:'0.14em'}}>{label}</div>
        <Avatar size="sm" emoji={emoji} />
      </div>
      <div className="font-mono font-bold leading-none"
           style={{fontSize:36,color:valueColor,textShadow:`0 0 12px ${valueColor}40`}}>
        {value}
      </div>
      <div className="font-mono text-text-2 mt-2" style={{fontSize:12}}>{sub}</div>
    </NeonCard>
  )
}

export default function Status() {
  const navigate = useNavigate()
  const { data: about } = useAbout()
  const backend = useBackend()
  const active = useUIStore((s) => s.activeProject())
  const [showLogs, setShowLogs] = useState(false)

  const onlineWorkers = backend.online_workers
  const isPrivate = active?.connectionMode === 'private'
  const ledgerSize = about?.ledger_tree_size ?? 0
  const uptime = about?.uptime_seconds ?? 0

  return (
    <>
      <div className="p-7 max-w-5xl">
        <SectionLabel>STATUS</SectionLabel>
        <HeroTitle accent="mesh">Your</HeroTitle>
        <p className="text-[16px] text-text-1 mt-2 mb-7">
          Live snapshot of boss, workers, relay, and ledger. Refreshes every 2 seconds.
        </p>

        {/* 4 stat cards */}
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3.5 mb-6">
          <StatCard
            label="BOSS" emoji="⚡"
            value={backend.ok ? 'UP' : 'DOWN'}
            valueTone={backend.ok ? 'ok' : 'rose'}
            sub={`running for ${formatUptime(uptime)}`}
          />
          <StatCard
            label="WORKERS" emoji="🛰"
            value={String(onlineWorkers)}
            valueTone="cyan"
            sub={`online · ${active ? 'live mesh' : 'no project'}`}
          />
          <StatCard
            label="RELAY" emoji={isPrivate ? '🔒' : '🌐'}
            value={isPrivate ? 'PSK' : 'PUBLIC'}
            valueTone={isPrivate ? 'violet' : 'cyan'}
            sub={about?.relay_peer_id ? shortenPeerID(about.relay_peer_id, 6, 5) : '(not connected)'}
          />
          <StatCard
            label="LEDGER" emoji="📜"
            value={String(ledgerSize)}
            valueTone="cyan"
            sub="ratings + comments signed"
          />
        </div>

        {/* Summary */}
        <NeonCard className="p-5 mb-6 flex items-center gap-5">
          <div className="relative w-20 h-20 rounded-full flex-shrink-0"
               style={{
                 border:'1px solid rgba(34,211,238,.4)',
                 background:'radial-gradient(circle, rgba(34,211,238,.08), transparent 70%)',
                 boxShadow:'inset 0 0 18px -4px rgba(34,211,238,.4)',
               }}>
            <span className="absolute inset-0 rounded-full animate-radar-sweep"
                  style={{background:'conic-gradient(from 0deg, transparent 0deg, rgba(34,211,238,.5) 60deg, transparent 90deg)'}}/>
            {Array.from({ length: Math.min(onlineWorkers, 3) }).map((_, i) => (
              <span key={i} className="absolute w-1.5 h-1.5 rounded-full bg-accent animate-pulse-cyan"
                    style={{
                      top: `${20 + i*25}%`, left: `${30 + i*15}%`,
                      boxShadow:'0 0 8px #22d3ee',
                      animationDelay: `${-i}s`,
                    }}/>
            ))}
          </div>
          <div>
            <h3 className="text-[22px] font-semibold tracking-[-0.01em] m-0">
              {backend.ok ? 'All systems are healthy' : 'Backend is down'}
              {backend.ok && (
                <span className="ml-2.5 inline-flex items-center gap-1.5 font-mono font-bold text-[12px]
                  px-2.5 py-0.5 rounded-full uppercase"
                  style={{
                    background:'rgba(132,204,22,.1)',color:'#84cc16',
                    border:'1px solid rgba(132,204,22,.3)',letterSpacing:'0.1em',
                  }}>
                  <span className="w-1.5 h-1.5 rounded-full bg-ok"
                        style={{boxShadow:'0 0 6px #84cc16'}}/>OK
                </span>
              )}
            </h3>
            <p className="text-[15px] text-text-1 mt-1 leading-[1.55]">
              You're talking to <b className="text-accent">{onlineWorkers} online worker{onlineWorkers === 1 ? '' : 's'}</b>{' '}
              through your <b className="text-accent2-light">{isPrivate ? 'private relay' : 'public relay'}</b>.{' '}
              Trust gate refuses anything below{' '}
              <span className="font-mono text-accent-high">{(active?.reputationFloor ?? -0.5).toFixed(2)}</span> honesty.
            </p>
          </div>
        </NeonCard>

        {/* Tech details */}
        <SectionLabel>TECHNICAL DETAILS</SectionLabel>
        <NeonCard className="p-5">
          <div className="space-y-2.5">
            <TechRow k="Boss peer ID" v={about?.boss_peer_id ?? '…'} tone="cyan" />
            <TechRow k="Relay multiaddr" v={about?.relay_multiaddr || '(not connected)'} tone="violet" />
            <TechRow k="Backend version" v={about?.version ?? '…'} />
            <TechRow k="Reputation floor" v={(active?.reputationFloor ?? -0.5).toFixed(2)} />
            <TechRow k="Ledger storage" v="~/.agentfm/" />
          </div>
          <div className="flex gap-2.5 mt-4 flex-wrap">
            <button onClick={() => setShowLogs(true)}
              className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-[13px] font-semibold
                text-text-0 border border-accent/30 hover:border-accent/55 transition-colors">
              <FileText size={12} /><span>View logs</span>
            </button>
            <button onClick={() => window.api.backend.restart()}
              className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-[13px] font-semibold
                text-text-0 border border-accent/30 hover:border-accent/55 transition-colors">
              <RefreshCw size={12} /><span>Restart backend</span>
            </button>
            <GradientButton onClick={() => navigate('/radar')}>
              <RadarIcon size={13} /><span>Open Radar</span>
            </GradientButton>
            <GradientButton variant="violet" onClick={() => navigate('/activity')}>
              <ScrollText size={13} /><span>See my activity</span>
            </GradientButton>
          </div>
        </NeonCard>
      </div>
      <LogsModal isOpen={showLogs} onClose={() => setShowLogs(false)} />
    </>
  )
}

function TechRow({ k, v, tone = 'default' }:
  { k: string; v: string; tone?: 'default' | 'cyan' | 'violet' }) {
  const color = tone === 'cyan' ? 'text-accent-high'
              : tone === 'violet' ? 'text-accent2-light'
              : 'text-text-1'
  return (
    <div className="flex items-baseline gap-3.5">
      <div className="w-[150px] flex-shrink-0 font-mono uppercase font-bold text-text-2"
           style={{fontSize:11,letterSpacing:'0.14em'}}>{k}</div>
      <div className={`flex-1 font-mono text-[13px] break-all ${color}`}>{v}</div>
    </div>
  )
}
