import { useEffect, useState } from 'react'
import { useUIStore } from '../lib/store'
import { useAbout } from '../lib/query'
import { Button } from '../components/primitives/Button'
import { SegGroup } from '../components/primitives/SegGroup'
import { Card } from '../components/primitives/Card'
import { toast } from 'sonner'

const DEFAULTS = {
  theme: 'dark' as const,
  accent: 'emerald' as const,
  apiPort: 8080,
  telemetry: false,
}

export default function Settings() {
  const ui = useUIStore()
  const { data: about } = useAbout()
  const [telemetry, setTelemetry] = useState(false)

  useEffect(() => {
    window.api?.settings.get<boolean>('telemetry').then((v) => setTelemetry(!!v))
  }, [])

  async function resetToDefaults() {
    if (!window.confirm('Reset appearance + telemetry to defaults?')) return
    ui.setTheme(DEFAULTS.theme)
    ui.setAccent(DEFAULTS.accent)
    ui.setApiPort(DEFAULTS.apiPort)
    setTelemetry(DEFAULTS.telemetry)
    await Promise.all([
      window.api.settings.set('theme', DEFAULTS.theme),
      window.api.settings.set('accent', DEFAULTS.accent),
      window.api.settings.set('apiPort', DEFAULTS.apiPort),
      window.api.settings.set('telemetry', DEFAULTS.telemetry),
    ])
    toast.success('App settings reset')
  }

  return (
    <div className="p-7 max-w-3xl">
      <h1 className="text-2xl font-semibold text-text-0">App settings</h1>
      <p className="text-text-2 mt-1 mb-6">
        Per-project mesh, relay, and trust threshold live on the project pill in the top bar.
      </p>

      <Card className="p-5 mb-5">
        <div className="text-xs uppercase tracking-wider text-text-0 font-semibold mb-1">Appearance</div>
        <div className="text-xs text-text-2 mb-4">Applies instantly — no restart.</div>
        <label className="block text-xs text-text-1 mb-1.5">Theme</label>
        <SegGroup
          options={[
            { value: 'dark', label: 'Dark' },
            { value: 'light', label: 'Light' },
            { value: 'auto', label: 'Auto' },
          ]}
          value={ui.theme}
          onChange={ui.setTheme}
        />
        <label className="block text-xs text-text-1 mt-4 mb-1.5">Accent</label>
        <div className="flex gap-3 items-center">
          {(['emerald', 'violet', 'rose'] as const).map((c) => (
            <button
              key={c}
              onClick={() => ui.setAccent(c)}
              className={`w-6 h-6 rounded-full transition-all ${
                ui.accent === c ? 'ring-2 ring-white' : 'opacity-70 hover:opacity-100'
              }`}
              style={{ background: c === 'emerald' ? '#10b981' : c === 'violet' ? '#8b5cf6' : '#f43f5e' }}
            />
          ))}
        </div>
      </Card>

      <Card className="p-5 mb-5">
        <div className="text-xs uppercase tracking-wider text-text-0 font-semibold mb-1">Advanced</div>
        <div className="text-xs text-text-2 mb-3">Backend: <span className="font-mono text-text-0">agentfm {about?.version ?? '…'}</span></div>
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
            <div className="text-2xs text-text-2">Crash counts and screen views only — never task contents.</div>
          </div>
        </label>
      </Card>

      <div className="flex justify-between items-center pt-4 border-t border-border-0">
        <Button variant="ghost" onClick={resetToDefaults}>Reset to defaults</Button>
      </div>
    </div>
  )
}
