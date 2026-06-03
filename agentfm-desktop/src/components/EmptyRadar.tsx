import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Button } from './primitives/Button';

const WORKER_CMD = `./agentfm -mode worker \\
  -agentdir ./agents/hello \\
  -image agentfm-hello:latest \\
  -model llama3.2 \\
  -agent "Hello Worker" \\
  -desc "Echoes prompts back" \\
  -maxtasks 5`;

export function EmptyRadar() {
  const navigate = useNavigate();
  const [copied, setCopied] = useState(false);

  function copyCmd() {
    navigator.clipboard
      .writeText(WORKER_CMD)
      .then(() => {
        setCopied(true);
        setTimeout(() => setCopied(false), 1800);
      })
      .catch(() => {
        // clipboard unavailable in some test envs — no-op
      });
  }

  return (
    <div className="max-w-2xl bg-bg-1 border border-border-0 rounded-xl p-7">
      <div className="text-5xl mb-3 opacity-70">🛰</div>
      <h2 className="text-xl font-semibold tracking-tight text-text-0">No agents on the mesh yet</h2>
      <p className="text-sm text-text-2 mt-1.5 mb-5">
        Workers announce themselves on the GossipSub telemetry topic. Once one
        comes online, its card will appear here in real time — no refresh needed.
      </p>

      <div className="space-y-4">
        <Step
          n={1}
          title="Start your first worker"
          body={
            <>
              <p className="text-xs text-text-2 mb-2">
                Run this in a separate terminal. It uses the bundled public relay by default.
              </p>
              <div className="relative">
                <pre className="text-[11px] font-mono bg-bg-0 border border-border-0 rounded-md p-3 overflow-x-auto text-text-1">
                  {WORKER_CMD}
                </pre>
                <button
                  onClick={copyCmd}
                  className="absolute top-2 right-2 text-[10px] bg-bg-2 border border-border-0 rounded px-2 py-0.5 text-text-2 hover:text-text-0"
                >
                  {copied ? 'copied!' : 'copy'}
                </button>
              </div>
            </>
          }
        />
        <Step
          n={2}
          title="Or point at your own relay"
          body={
            <>
              <p className="text-xs text-text-2 mb-2">
                Running a private mesh? Configure the relay multiaddr and (optionally) a swarm
                key in Settings → Mesh.
              </p>
              <Button onClick={() => navigate('/settings')}>Open Mesh settings</Button>
            </>
          }
        />
        <Step
          n={3}
          title="Still nothing?"
          body={
            <p className="text-xs text-text-2">
              Check the relay is reachable from this machine (firewalls, NAT) and that the
              worker process hasn't exited. The Status page surfaces backend health.
            </p>
          }
        />
      </div>
    </div>
  );
}

function Step({
  n,
  title,
  body,
}: {
  n: number;
  title: string;
  body: React.ReactNode;
}) {
  return (
    <div className="flex gap-3">
      <div className="w-6 h-6 shrink-0 rounded-full bg-accent-bg border border-accent/30 text-accent text-xs font-semibold flex items-center justify-center">
        {n}
      </div>
      <div className="flex-1 min-w-0">
        <div className="text-sm font-medium text-text-0 mb-1">{title}</div>
        {body}
      </div>
    </div>
  );
}
