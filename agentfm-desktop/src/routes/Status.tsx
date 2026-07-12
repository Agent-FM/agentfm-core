import { motion } from 'framer-motion'
import { Zap, Lock, Globe, AlertTriangle, Satellite, ScrollText } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useAbout } from '../lib/query'
import { useBackend } from '../hooks/useBackend'
import { useUIStore } from '../lib/store'
import { HeroTitle } from '../components/primitives/HeroTitle'
import { Card } from '../components/primitives/Card'
import { Avatar } from '../components/primitives/Avatar'
import { StatusDot } from '../components/primitives/StatusDot'
import { staggerItem } from '../lib/motion'
import { shortenPeerID } from '../lib/peer'
import { COLORS } from '../lib/colors'

function formatUptime(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = seconds % 60
  return [h, m, s].map((n) => String(n).padStart(2, '0')).join(':')
}

interface StatCardProps {
  label: string
  icon: LucideIcon
  value: string
  valueTone: 'ok' | 'accent' | 'bad'
  sub: string
}

function StatCard({ label, icon: Icon, value, valueTone, sub }: StatCardProps) {
  const valueColor = {
    ok: COLORS.ok, accent: COLORS.accent, bad: COLORS.bad,
  }[valueTone]
  return (
    <Card>
      <div className="flex items-center justify-between mb-2">
        <div className="text-2xs font-medium text-text-2">{label}</div>
        <Avatar size="sm">
          <Icon size={13} strokeWidth={1.5} />
        </Avatar>
      </div>
      <div className="text-lg font-mono font-semibold leading-none tabular-nums"
           style={{color:valueColor}}>
        {value}
      </div>
      <div className="text-2xs font-mono text-text-2 mt-1.5 truncate tabular-nums">{sub}</div>
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
  const relayValue = !relayConnected ? 'Off' : isPrivate ? 'Private' : 'Public'
  const relayTone: 'ok' | 'accent' | 'bad' = !relayConnected ? 'bad' : 'accent'
  const relayIcon = !relayConnected ? AlertTriangle : isPrivate ? Lock : Globe

  const healthy = backend.ok && relayConnected

  return (
    <div className="p-4 max-w-5xl">
      <HeroTitle accent="status">System</HeroTitle>

      {/* Health banner */}
      <motion.div {...staggerItem(0)}>
        <Card
          className={`mt-4 mb-2 flex items-center gap-3 ${
            healthy ? '' : 'border-bad/40'
          }`}
        >
          <StatusDot tone={healthy ? 'accent' : 'bad'} pulse={healthy} size="md" />
          <div className="min-w-0">
            <div className="text-sm font-semibold text-text-0">
              {backend.ok
                ? healthy
                  ? 'All systems healthy'
                  : 'Backend up, relay not connected'
                : 'Backend is down'}
            </div>
            <div className="flex items-center gap-3 text-xs text-text-2 mt-0.5 font-mono tabular-nums">
              <span>{onlineWorkers} agent{onlineWorkers === 1 ? '' : 's'} online</span>
            </div>
          </div>
        </Card>
      </motion.div>

      {/* Health cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        {[
          <StatCard
            key="boss"
            label="Boss" icon={Zap}
            value={backend.ok ? 'Up' : 'Down'}
            valueTone={backend.ok ? 'ok' : 'bad'}
            sub={`running for ${formatUptime(uptime)}`}
          />,
          <StatCard
            key="agents"
            label="Agents" icon={Satellite}
            value={String(onlineWorkers)}
            valueTone="accent"
            sub={active ? 'live mesh' : 'no project'}
          />,
          <StatCard
            key="relay"
            label="Relay" icon={relayIcon}
            value={relayValue}
            valueTone={relayTone}
            sub={about?.relay_peer_id ? shortenPeerID(about.relay_peer_id, 6, 5) : 'Not connected'}
          />,
          <StatCard
            key="ledger"
            label="Ledger" icon={ScrollText}
            value={String(ledgerSize)}
            valueTone="accent"
            sub="ratings and comments signed"
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
