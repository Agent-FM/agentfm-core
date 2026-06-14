import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { motion } from 'framer-motion'
import { FileText, RefreshCw, Radar as RadarIcon, ScrollText } from 'lucide-react'
import { toast } from 'sonner'
import { useAbout } from '../lib/query'
import { useBackend } from '../hooks/useBackend'
import { useUIStore } from '../lib/store'
import { SectionLabel } from '../components/primitives/SectionLabel'
import { Card } from '../components/primitives/Card'
import { Avatar } from '../components/primitives/Avatar'
import { Button } from '../components/primitives/Button'
import { staggerItem } from '../lib/motion'
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
    ok: '#84cc16', cyan: '#22d3ee', violet: '#22d3ee', rose: '#f43f5e',
  }[valueTone]
  return (
    <Card live className="p-3">
      <div className="flex items-center justify-between mb-2">
        <div className="font-mono uppercase font-bold text-text-2"
             style={{fontSize:10,letterSpacing:'0.14em'}}>{label}</div>
        <Avatar size="sm" emoji={emoji} />
      </div>
      <div className="font-mono font-bold leading-none tabular-nums"
           style={{fontSize:28,color:valueColor}}>
        {value}
      </div>
      <div className="font-mono text-text-2 mt-1.5 truncate tabular-nums" style={{fontSize:11}}>{sub}</div>
    </Card>
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

  const relayConnected = !!about?.relay_peer_id
  const relayValue = !relayConnected ? 'OFF' : isPrivate ? 'PSK' : 'PUBLIC'
  const relayTone: 'ok' | 'cyan' | 'violet' | 'rose' = !relayConnected
    ? 'rose'
    : 'cyan'
  const relayEmoji = !relayConnected ? '⚠' : isPrivate ? '🔒' : '🌐'

  async function handleRestart() {
    const ok = window.confirm('Restart the backend? Any in-flight tasks will be cancelled.')
    if (!ok) return
    try {
      await window.api.backend.restart()
      toast.success('Backend restarted')
    } catch (e) {
      toast.error('Restart failed: ' + (e as Error).message)
    }
  }

  return (
    <>
      <div className="p-6 max-w-5xl">
        <div className="flex items-baseline justify-between mb-6">
          <SectionLabel>STATUS</SectionLabel>
          <div className="text-[12px] text-text-2">
            {backend.ok ? 'All systems healthy' : 'Backend is down'} ·
            <b className="text-accent ml-1 tabular-nums">{onlineWorkers} agent{onlineWorkers === 1 ? '' : 's'} online</b> ·
            <span className="text-text-2 ml-1">{isPrivate ? 'private relay' : 'public relay'}</span> ·
            <span className="text-text-2 ml-1 tabular-nums">trust floor {(active?.reputationFloor ?? -0.5).toFixed(2)}</span>
          </div>
        </div>

        {/* 4 stat cards */}
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
          {[
            <StatCard
              key="boss"
              label="BOSS" emoji="⚡"
              value={backend.ok ? 'UP' : 'DOWN'}
              valueTone={backend.ok ? 'ok' : 'rose'}
              sub={`running for ${formatUptime(uptime)}`}
            />,
            <StatCard
              key="agents"
              label="AGENTS" emoji="🛰"
              value={String(onlineWorkers)}
              valueTone="cyan"
              sub={`online · ${active ? 'live mesh' : 'no project'}`}
            />,
            <StatCard
              key="relay"
              label="RELAY" emoji={relayEmoji}
              value={relayValue}
              valueTone={relayTone}
              sub={about?.relay_peer_id ? shortenPeerID(about.relay_peer_id, 6, 5) : '(not connected)'}
            />,
            <StatCard
              key="ledger"
              label="LEDGER" emoji="📜"
              value={String(ledgerSize)}
              valueTone="cyan"
              sub="ratings + comments signed"
            />,
          ].map((card, i) => (
            <motion.div key={card.key} {...staggerItem(i)}>
              {card}
            </motion.div>
          ))}
        </div>

        {/* Tech details */}
        <Card className="p-4">
          <div className="grid grid-cols-2 gap-x-6 gap-y-1.5 mb-3">
            <TechRow k="Boss peer ID" v={about?.boss_peer_id ?? '…'} tone="cyan" />
            <TechRow k="Backend version" v={about?.version ?? '…'} />
            <TechRow k="Relay multiaddr" v={about?.relay_multiaddr || '(not connected)'} tone="cyan" />
            <TechRow k="Ledger storage" v="~/.agentfm/" />
          </div>
          <div className="flex gap-2 flex-wrap pt-3 border-t border-border-0">
            <Button variant="primary" onClick={() => navigate('/radar')}>
              <RadarIcon size={13} /><span>Open Radar</span>
            </Button>
            <Button variant="secondary" onClick={() => navigate('/activity')}>
              <ScrollText size={13} /><span>See my activity</span>
            </Button>
            <Button variant="ghost" onClick={() => setShowLogs(true)}>
              <FileText size={12} /><span>View logs</span>
            </Button>
            <Button variant="ghost" onClick={handleRestart}>
              <RefreshCw size={12} /><span>Restart backend</span>
            </Button>
          </div>
        </Card>
      </div>
      <LogsModal isOpen={showLogs} onClose={() => setShowLogs(false)} />
    </>
  )
}

function TechRow({ k, v, tone = 'default' }:
  { k: string; v: string; tone?: 'default' | 'cyan' }) {
  const color = tone === 'cyan' ? 'text-accent-high' : 'text-text-1'
  return (
    <div className="flex items-baseline gap-2.5 min-w-0">
      <div className="w-[110px] flex-shrink-0 font-mono uppercase font-bold text-text-2"
           style={{fontSize:10,letterSpacing:'0.12em'}}>{k}</div>
      <div className={`flex-1 font-mono text-[12px] truncate tabular-nums ${color}`} title={v}>{v}</div>
    </div>
  )
}
