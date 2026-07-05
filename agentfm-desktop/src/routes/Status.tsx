import { motion } from 'framer-motion'
import { useAbout } from '../lib/query'
import { useBackend } from '../hooks/useBackend'
import { useUIStore } from '../lib/store'
import { SectionLabel } from '../components/primitives/SectionLabel'
import { Card } from '../components/primitives/Card'
import { Avatar } from '../components/primitives/Avatar'
import { StatusDot } from '../components/primitives/StatusDot'
import { staggerItem } from '../lib/motion'
import { shortenPeerID } from '../lib/peer'

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
    ok: '#84cc16', cyan: '#F7931E', violet: '#F7931E', rose: '#f43f5e',
  }[valueTone]
  return (
    <Card live className="p-4">
      <div className="flex items-center justify-between mb-3">
        <div className="font-mono uppercase font-bold text-text-2"
             style={{fontSize:10,letterSpacing:'0.14em'}}>{label}</div>
        <Avatar size="sm" emoji={emoji} />
      </div>
      <div className="font-mono font-bold leading-none tabular-nums"
           style={{fontSize:30,color:valueColor}}>
        {value}
      </div>
      <div className="font-mono text-text-2 mt-2 truncate tabular-nums" style={{fontSize:11}}>{sub}</div>
    </Card>
  )
}

export default function Status() {
  const { data: about } = useAbout()
  const backend = useBackend()
  const active = useUIStore((s) => s.activeProject())

  const onlineWorkers = backend.online_workers
  const isPrivate = active?.connectionMode === 'private'
  const ledgerSize = about?.ledger_tree_size ?? 0
  const uptime = about?.uptime_seconds ?? 0

  const relayConnected = !!about?.relay_peer_id
  const relayValue = !relayConnected ? 'OFF' : isPrivate ? 'PSK' : 'PUBLIC'
  const relayTone: 'ok' | 'cyan' | 'violet' | 'rose' = !relayConnected ? 'rose' : 'cyan'
  const relayEmoji = !relayConnected ? '⚠' : isPrivate ? '🔒' : '🌐'

  const healthy = backend.ok && relayConnected

  return (
    <div className="p-6 max-w-5xl">
      <SectionLabel>STATUS</SectionLabel>

      {/* Health banner */}
      <motion.div {...staggerItem(0)}>
        <Card
          live={healthy}
          className={`mt-2 mb-5 p-5 flex items-center gap-4 ${
            healthy ? '' : 'border-bad/40'
          }`}
        >
          <StatusDot tone={healthy ? 'cyan' : 'rose'} pulse={healthy} size="lg" />
          <div className="min-w-0">
            <div className="text-[20px] font-semibold tracking-tight text-text-0">
              {backend.ok
                ? healthy
                  ? 'All systems healthy'
                  : 'Backend up — relay not connected'
                : 'Backend is down'}
            </div>
            <div className="text-[13px] text-text-2 mt-0.5 font-mono">
              {onlineWorkers} agent{onlineWorkers === 1 ? '' : 's'} online ·{' '}
              {isPrivate ? 'private swarm' : 'public mesh'} · up {formatUptime(uptime)}
            </div>
          </div>
        </Card>
      </motion.div>

      {/* Health cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
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
            sub={active ? 'online · live mesh' : 'no project'}
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
          <motion.div key={card.key} {...staggerItem(i + 1)}>
            {card}
          </motion.div>
        ))}
      </div>
    </div>
  )
}
