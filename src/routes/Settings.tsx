import { useEffect, useState } from 'react'
import { useUIStore } from '../lib/store'
import { useAbout } from '../lib/query'
import { Button } from '../components/primitives/Button'
import { Input } from '../components/primitives/Input'
import { Slider } from '../components/primitives/Slider'
import { SegGroup } from '../components/primitives/SegGroup'
import { Card } from '../components/primitives/Card'
import { toast } from 'sonner'
import { api, ApiError } from '../lib/api'

// /ip4/X/tcp/Y/p2p/Z and /dns/host/tcp/Y/p2p/Z are the two shapes we
// care about for relays. Accept either; reject obvious typos.
const MULTIADDR_RE = /^\/(ip4|ip6|dns|dns4|dns6)\/[^/]+\/tcp\/\d+\/p2p\/[A-Za-z0-9]+$/

const DEFAULTS = {
  theme: 'dark' as const,
  accent: 'emerald' as const,
  apiPort: 8080,
  reputationFloor: -0.5,
  relayMultiaddr: null as string | null,
  telemetry: false,
}

export default function Settings() {
  const ui = useUIStore()
  const { data: about } = useAbout()

  // Draft state for restart-required fields
  const [draftRelay, setDraftRelay] = useState(ui.relayMultiaddr ?? '')
  const [draftPort, setDraftPort] = useState(ui.apiPort)
  const [draftFloor, setDraftFloor] = useState(ui.reputationFloor)
  const [telemetry, setTelemetry] = useState(false)

  useEffect(() => {
    window.api?.settings.get<boolean>('telemetry').then((v) => setTelemetry(!!v))
  }, [])

  const dirty =
    draftRelay !== (ui.relayMultiaddr ?? '') ||
    draftPort !== ui.apiPort ||
    draftFloor !== ui.reputationFloor

  async function testConnection() {
    if (draftRelay && !MULTIADDR_RE.test(draftRelay)) {
      toast.error('That doesn’t look like a multiaddr. Expected /ip4/.../tcp/.../p2p/...')
      return
    }
    if (draftRelay && draftRelay !== (ui.relayMultiaddr ?? '')) {
      toast.message('Looks valid. Save & restart backend to dial it.')
      return
    }
    try {
      const a = await api.about()
      if (a.relay_peer_id) {
        toast.success(
          `Connected to relay ${a.relay_peer_id.slice(0, 12)}… (${a.relay_multiaddr || 'default lighthouse'})`,
        )
      } else {
        toast.error('Backend is up but not connected to a relay yet — check firewall/NAT.')
      }
    } catch (err) {
      const msg = err instanceof ApiError ? `${err.status}: ${err.message}` : (err as Error).message
      toast.error('Could not query backend: ' + msg)
    }
  }

  async function saveAndRestart() {
    ui.setRelayMultiaddr(draftRelay || null)
    ui.setApiPort(draftPort)
    ui.setReputationFloor(draftFloor)
    await Promise.all([
      window.api.settings.set('relayMultiaddr', draftRelay || null),
      window.api.settings.set('apiPort', draftPort),
      window.api.settings.set('reputationFloor', draftFloor),
    ])
    try {
      await window.api.backend.restart({
        apiPort: draftPort,
        reputationFloor: draftFloor,
        relayMultiaddr: draftRelay || undefined,
      })
      toast.success('Backend restarted with new settings')
    } catch (err) {
      toast.error('Failed to restart backend: ' + (err as Error).message)
    }
  }

  async function resetToDefaults() {
    if (!window.confirm('Reset all settings to defaults? This will restart the backend if mesh, port, or trust floor change.')) {
      return
    }
    const meshChanged =
      ui.relayMultiaddr !== DEFAULTS.relayMultiaddr ||
      ui.apiPort !== DEFAULTS.apiPort ||
      ui.reputationFloor !== DEFAULTS.reputationFloor

    ui.setTheme(DEFAULTS.theme)
    ui.setAccent(DEFAULTS.accent)
    ui.setApiPort(DEFAULTS.apiPort)
    ui.setReputationFloor(DEFAULTS.reputationFloor)
    ui.setRelayMultiaddr(DEFAULTS.relayMultiaddr)
    setDraftRelay('')
    setDraftPort(DEFAULTS.apiPort)
    setDraftFloor(DEFAULTS.reputationFloor)
    setTelemetry(DEFAULTS.telemetry)
    await Promise.all([
      window.api.settings.set('theme', DEFAULTS.theme),
      window.api.settings.set('accent', DEFAULTS.accent),
      window.api.settings.set('apiPort', DEFAULTS.apiPort),
      window.api.settings.set('reputationFloor', DEFAULTS.reputationFloor),
      window.api.settings.set('relayMultiaddr', DEFAULTS.relayMultiaddr),
      window.api.settings.set('telemetry', DEFAULTS.telemetry),
    ])

    if (meshChanged) {
      try {
        await window.api.backend.restart({
          apiPort: DEFAULTS.apiPort,
          reputationFloor: DEFAULTS.reputationFloor,
          relayMultiaddr: undefined,
        })
        toast.success('Settings reset and backend restarted')
      } catch (err) {
        toast.error('Reset done, but backend restart failed: ' + (err as Error).message)
      }
    } else {
      toast.success('Settings reset to defaults')
    }
  }

  return (
    <div className="p-7 max-w-3xl">
      <h1 className="text-xl font-semibold text-text-0">Settings</h1>
      <p className="text-sm text-text-2 mt-1 mb-6">
        Mesh connection, trust thresholds, and appearance.
      </p>

      <Card className="p-5 mb-5">
        <SectionHeader
          title="Mesh"
          hint="Where the bundled agentfm backend should bootstrap from."
        />
        <Field label="Relay multiaddr">
          <div className="flex gap-2">
            <Input
              className="flex-1 font-mono text-xs"
              placeholder="/ip4/127.0.0.1/tcp/4001/p2p/12D3Koo…"
              value={draftRelay}
              onChange={(e) => setDraftRelay(e.target.value)}
            />
            <Button onClick={testConnection}>Test connection</Button>
          </div>
        </Field>
        <Field label="Backend port (HTTP API)">
          <Input
            type="number"
            value={draftPort}
            onChange={(e) => setDraftPort(Number(e.target.value))}
            className="w-32"
          />
        </Field>
      </Card>

      <Card className="p-5 mb-5">
        <SectionHeader
          title="Trust"
          hint="Workers below this honesty score are auto-refused on dispatch."
        />
        <Field label="Reputation floor">
          <Slider min={-1} max={0} step={0.05} value={draftFloor} onChange={setDraftFloor} />
          <div className="flex justify-between text-[10px] text-text-2 mt-1 font-mono">
            <span>-1.0 (allow all)</span>
            <span>{draftFloor.toFixed(2)}</span>
            <span>0.0</span>
          </div>
        </Field>
      </Card>

      <Card className="p-5 mb-5">
        <SectionHeader title="Appearance" hint="Applies instantly — no restart." />
        <Field label="Theme">
          <SegGroup
            options={[
              { value: 'dark', label: 'Dark' },
              { value: 'light', label: 'Light' },
              { value: 'auto', label: 'Auto' },
            ]}
            value={ui.theme}
            onChange={ui.setTheme}
          />
        </Field>
        <Field label="Accent">
          <div className="flex gap-3 items-center">
            {(['emerald', 'violet', 'rose'] as const).map((color) => (
              <button
                key={color}
                onClick={() => ui.setAccent(color)}
                className={`w-6 h-6 rounded-full transition-all ${
                  ui.accent === color ? 'ring-2 ring-white' : 'opacity-70 hover:opacity-100'
                }`}
                style={{
                  background:
                    color === 'emerald' ? '#10b981' : color === 'violet' ? '#8b5cf6' : '#f43f5e',
                }}
                title={color}
              />
            ))}
            <span className="text-xs text-text-2 ml-2">{ui.accent}</span>
          </div>
        </Field>
      </Card>

      <Card className="p-5 mb-5">
        <SectionHeader title="Advanced" hint="Bundled binary version is fixed per release." />
        <div className="text-sm text-text-1 mb-3">
          Backend:{' '}
          <span className="font-mono text-text-0">agentfm {about?.version ?? '…'}</span>
        </div>
        <label className="flex items-center gap-3 text-sm text-text-1 cursor-pointer">
          <input
            type="checkbox"
            checked={telemetry}
            onChange={async (e) => {
              setTelemetry(e.target.checked)
              await window.api.settings.set('telemetry', e.target.checked)
            }}
          />
          <div>
            Send anonymized usage telemetry
            <div className="text-[11px] text-text-2">
              Off by default. Crash counts and screen views only — never task contents.
            </div>
          </div>
        </label>
      </Card>

      <div className="flex justify-between items-center pt-4 border-t border-border-0">
        <Button variant="ghost" onClick={resetToDefaults}>
          Reset all to defaults
        </Button>
        <div className="flex gap-2">
          <Button variant="primary" onClick={saveAndRestart} disabled={!dirty}>
            {dirty ? 'Save & restart backend' : 'No pending changes'}
          </Button>
        </div>
      </div>
    </div>
  )
}

function SectionHeader({ title, hint }: { title: string; hint: string }) {
  return (
    <>
      <div className="text-xs uppercase tracking-wider text-text-0 font-semibold mb-1">
        {title}
      </div>
      <div className="text-xs text-text-2 mb-4">{hint}</div>
    </>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="mb-4">
      <label className="block text-xs text-text-1 mb-1.5">{label}</label>
      {children}
    </div>
  )
}
