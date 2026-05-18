import { useEffect, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Button } from './primitives/Button';
import { Input } from './primitives/Input';
import { useUIStore } from '../lib/store';
import { toast } from 'sonner';

type Step = 'intro' | 'mesh' | 'done';

const MULTIADDR_RE = /^\/(ip4|ip6|dns|dns4|dns6)\/[^/]+\/tcp\/\d+\/p2p\/[A-Za-z0-9]+$/;

export function WelcomeModal() {
  const [open, setOpen] = useState(false);
  const [step, setStep] = useState<Step>('intro');
  const [useDefault, setUseDefault] = useState(true);
  const [customRelay, setCustomRelay] = useState('');
  const [saving, setSaving] = useState(false);
  const ui = useUIStore();

  useEffect(() => {
    // Decide whether to show the modal once on mount.
    if (!window.api?.settings) return;
    window.api.settings
      .get<boolean>('hasOnboarded')
      .then((v) => {
        if (!v) setOpen(true);
      })
      .catch(() => {
        // No store available (test env) — leave closed.
      });
  }, []);

  async function finish(skipped: boolean) {
    setSaving(true);
    try {
      if (!skipped && !useDefault) {
        if (!MULTIADDR_RE.test(customRelay.trim())) {
          toast.error('That doesn’t look like a multiaddr. Skipping for now — you can set it later in Settings.');
        } else {
          const v = customRelay.trim();
          ui.setRelayMultiaddr(v);
          await window.api?.settings.set('relayMultiaddr', v);
          try {
            await window.api?.backend.restart({
              apiPort: ui.apiPort,
              reputationFloor: ui.reputationFloor,
              relayMultiaddr: v,
            });
            toast.success('Connecting via your relay…');
          } catch (err) {
            toast.error('Saved, but backend restart failed: ' + (err as Error).message);
          }
        }
      }
      await window.api?.settings.set('hasOnboarded', true);
      setOpen(false);
    } finally {
      setSaving(false);
    }
  }

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.15 }}
          className="fixed inset-0 bg-black/70 backdrop-blur-sm z-[70] flex items-center justify-center"
        >
          <motion.div
            initial={{ scale: 0.96, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            exit={{ scale: 0.96, opacity: 0 }}
            transition={{ type: 'spring', stiffness: 380, damping: 28 }}
            className="w-[520px] bg-bg-1 border border-border-0 rounded-xl p-7 shadow-2xl"
          >
            <Stepper step={step} />

            {step === 'intro' && (
              <>
                <div className="text-5xl mb-3">🛰</div>
                <h2 className="text-xl font-semibold text-text-0">Welcome to AgentFM</h2>
                <p className="text-sm text-text-1 mt-2 leading-relaxed">
                  A peer-to-peer mesh for running containerized AI agents on idle hardware. This
                  desktop app is your <em>boss</em> node — it discovers workers, dispatches tasks,
                  and signs feedback into a tamper-evident ledger.
                </p>
                <p className="text-[12px] text-text-2 mt-3">Two quick questions and you’re in.</p>

                <div className="flex justify-between mt-6">
                  <Button variant="ghost" onClick={() => finish(true)} disabled={saving}>
                    Skip
                  </Button>
                  <Button variant="primary" onClick={() => setStep('mesh')}>
                    Get started →
                  </Button>
                </div>
              </>
            )}

            {step === 'mesh' && (
              <>
                <h2 className="text-lg font-semibold text-text-0">How should you connect?</h2>
                <p className="text-sm text-text-2 mt-1 mb-4">
                  Workers find each other through a libp2p relay. Pick one:
                </p>

                <label
                  className={`block border rounded-lg p-3 mb-3 cursor-pointer transition-colors ${
                    useDefault
                      ? 'border-accent/40 bg-accent-bg'
                      : 'border-border-0 bg-bg-2 hover:border-border-1'
                  }`}
                >
                  <div className="flex items-start gap-2">
                    <input
                      type="radio"
                      checked={useDefault}
                      onChange={() => setUseDefault(true)}
                      className="mt-1 accent-accent"
                    />
                    <div>
                      <div className="text-sm font-medium text-text-0">
                        Use the bundled public lighthouse{' '}
                        <span className="text-[10px] text-accent ml-1">recommended</span>
                      </div>
                      <div className="text-[11px] text-text-2 mt-0.5">
                        Easiest path. You’ll see every worker that joins the public mesh.
                      </div>
                    </div>
                  </div>
                </label>

                <label
                  className={`block border rounded-lg p-3 cursor-pointer transition-colors ${
                    !useDefault
                      ? 'border-accent/40 bg-accent-bg'
                      : 'border-border-0 bg-bg-2 hover:border-border-1'
                  }`}
                >
                  <div className="flex items-start gap-2">
                    <input
                      type="radio"
                      checked={!useDefault}
                      onChange={() => setUseDefault(false)}
                      className="mt-1 accent-accent"
                    />
                    <div className="flex-1">
                      <div className="text-sm font-medium text-text-0">Use my own relay</div>
                      <div className="text-[11px] text-text-2 mt-0.5">
                        For private meshes. Paste a multiaddr below.
                      </div>
                      {!useDefault && (
                        <Input
                          className="mt-2 font-mono text-xs"
                          placeholder="/ip4/198.51.100.55/tcp/4001/p2p/12D3KooW…"
                          value={customRelay}
                          onChange={(e) => setCustomRelay(e.target.value)}
                        />
                      )}
                    </div>
                  </div>
                </label>

                <div className="flex justify-between mt-6">
                  <Button onClick={() => setStep('intro')} disabled={saving}>
                    ← Back
                  </Button>
                  <Button variant="primary" onClick={() => setStep('done')} disabled={saving}>
                    Continue →
                  </Button>
                </div>
              </>
            )}

            {step === 'done' && (
              <>
                <div className="text-5xl mb-3">✅</div>
                <h2 className="text-xl font-semibold text-text-0">You’re ready.</h2>
                <p className="text-sm text-text-1 mt-2 leading-relaxed">
                  Workers will appear on the Radar as they join. From there you can dispatch tasks,
                  drill into peer history, or leave feedback that’s signed by your boss identity.
                </p>
                <ul className="text-xs text-text-2 mt-3 space-y-1.5 list-disc list-inside">
                  <li>
                    Press{' '}
                    <kbd className="font-mono bg-bg-2 border border-border-0 rounded px-1.5 py-0.5">
                      Cmd+1..5
                    </kbd>{' '}
                    to jump between Radar, Chat, Activity, Status, Settings.
                  </li>
                  <li>Status page shows backend health and the trust gate threshold.</li>
                  <li>Settings → Mesh lets you change relay or trust floor any time.</li>
                </ul>

                <div className="flex justify-between mt-6">
                  <Button onClick={() => setStep('mesh')} disabled={saving}>
                    ← Back
                  </Button>
                  <Button variant="primary" onClick={() => finish(false)} disabled={saving}>
                    {saving ? 'Saving…' : 'Take me to the Radar'}
                  </Button>
                </div>
              </>
            )}
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}

function Stepper({ step }: { step: Step }) {
  const steps: Step[] = ['intro', 'mesh', 'done'];
  const idx = steps.indexOf(step);
  return (
    <div className="flex gap-1.5 mb-5">
      {steps.map((s, i) => (
        <div
          key={s}
          className={`h-1 flex-1 rounded-full transition-colors ${
            i <= idx ? 'bg-accent' : 'bg-bg-2'
          }`}
        />
      ))}
    </div>
  );
}
