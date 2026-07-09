import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Satellite } from 'lucide-react';
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
        // clipboard unavailable in some test envs, no-op
      });
  }

  return (
    <div className="max-w-xl mx-auto px-4 pt-10 pb-6">
      <div className="flex flex-col items-center text-center mb-6">
        <Satellite size={32} strokeWidth={1.5} className="text-text-3 mb-3" aria-hidden="true" />
        <h2 className="text-lg font-semibold text-text-1">No agents on the mesh yet</h2>
        <p className="text-sm text-text-3 mt-1">
          Workers announce themselves on the GossipSub telemetry topic. Once one
          comes online, its card will appear here in real time, no refresh needed.
        </p>
      </div>

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
                <pre className="text-xs font-mono console-well mono-console rounded-ctl p-3 overflow-x-auto text-text-1">
                  {WORKER_CMD}
                </pre>
                <button
                  onClick={copyCmd}
                  className="absolute top-2 right-2 text-2xs bg-control hover:bg-control-hover rounded-ctl px-2 py-0.5 text-text-1 hover:text-text-0 transition-colors duration-150 cursor-pointer"
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
              <Button variant="secondary" onClick={() => navigate('/settings')}>Open Mesh settings</Button>
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
      <div className="w-5 h-5 shrink-0 rounded-full bg-white/[0.06] text-text-1 text-2xs font-medium tabular-nums flex items-center justify-center">
        {n}
      </div>
      <div className="flex-1 min-w-0">
        <div className="text-sm font-medium text-text-0 mb-1">{title}</div>
        {body}
      </div>
    </div>
  );
}
