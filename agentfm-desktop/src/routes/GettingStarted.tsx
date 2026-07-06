import { useState } from 'react'
import { Copy, Check, Rocket, Globe, Lock } from 'lucide-react'
import { toast } from 'sonner'
import { SectionLabel } from '../components/primitives/SectionLabel'
import { HeroTitle } from '../components/primitives/HeroTitle'
import { Card } from '../components/primitives/Card'

function Cmd({ code }: { code: string }) {
  const [copied, setCopied] = useState(false)
  const copy = () => {
    navigator.clipboard
      .writeText(code)
      .then(() => {
        setCopied(true)
        toast.success('Copied')
        setTimeout(() => setCopied(false), 1200)
      })
      .catch(() => toast.error('Copy failed'))
  }
  return (
    <div className="relative group/cmd my-2">
      <pre className="bg-bg-0 border border-border-0 rounded-xl p-3.5 pr-10 overflow-x-auto text-[12.5px] leading-[1.55] font-mono text-text-1 whitespace-pre">
        {code}
      </pre>
      <button
        onClick={copy}
        title="Copy"
        className="absolute top-2.5 right-2.5 p-1.5 rounded-lg text-text-2 hover:text-accent hover:bg-accent/10 transition-colors"
      >
        {copied ? <Check size={14} /> : <Copy size={14} />}
      </button>
    </div>
  )
}

function Step({ n, title, children }: { n: number; title: string; children: React.ReactNode }) {
  return (
    <div className="flex gap-3.5">
      <div className="shrink-0 w-7 h-7 rounded-full bg-accent/12 text-accent font-mono text-[13px] font-bold grid place-items-center mt-0.5">
        {n}
      </div>
      <div className="min-w-0 flex-1">
        <div className="text-[15px] font-semibold text-text-0 mb-1">{title}</div>
        <div className="text-[13px] text-text-1 leading-[1.55]">{children}</div>
      </div>
    </div>
  )
}

export default function GettingStarted() {
  return (
    <div className="p-6 max-w-4xl">
      <SectionLabel>GETTING STARTED</SectionLabel>
      <HeroTitle accent="agents">Run your</HeroTitle>
      <p className="text-[15px] text-text-1 mt-2 mb-6 leading-[1.6]">
        This app is your <b>Boss</b> — it discovers agents, dispatches tasks, and collects results.
        To put an agent <i>on</i> the mesh you run a <b>worker</b> from the terminal. It advertises
        itself, and it shows up here on the Radar within seconds.
      </p>

      <Card density="spacious" className="mb-5">
        <div className="flex items-center gap-2 mb-3">
          <Rocket size={16} className="text-accent" />
          <h2 className="text-[16px] font-semibold text-text-0">Prerequisites</h2>
        </div>
        <div className="space-y-3.5">
          <Step n={1} title="Podman — agents run in sandboxed containers">
            <Cmd code={`# macOS\nbrew install podman && podman machine init && podman machine start`} />
          </Step>
          <Step n={2} title="Ollama + a local model (for LLM agents)">
            <Cmd code={`curl -fsSL https://ollama.com/install.sh | sh && ollama run llama3.2`} />
          </Step>
          <Step n={3} title="The agentfm CLI (for running workers)">
            The desktop app already bundles the backend; grab the CLI too so you can launch workers.
            <Cmd code={`curl -fsSL https://api.agentfm.net/install.sh | bash`} />
          </Step>
        </div>
      </Card>

      <Card density="spacious" className="mb-5">
        <div className="flex items-center gap-2 mb-1">
          <Globe size={16} className="text-accent" />
          <h2 className="text-[16px] font-semibold text-text-0">Public agent</h2>
        </div>
        <p className="text-[13px] text-text-2 mb-3.5">
          Joins the public mesh automatically — no relay, no key. Anyone can dispatch to it;
          reputation accrues over time.
        </p>
        <div className="space-y-3.5">
          <Step n={1} title="Run a worker">
            Point it at your agent’s container image. <code className="text-accent">-capability</code>{' '}
            is a searchable tag (e.g. <code>research</code>, <code>code-review</code>).
            <Cmd
              code={`agentfm -mode worker \\
  -agentdir ./my-agent \\
  -image localhost/my-agent:latest \\
  -agent "My Agent" \\
  -capability "research" \\
  -model llama3.2 \\
  -maxtasks 4`}
            />
          </Step>
          <Step n={2} title="Find it in the app">
            Switch the project dropdown (top-left) to <b>Public agents</b>. Your worker appears on the{' '}
            <b>Radar</b> within a few seconds — click <b>Dispatch</b> to send it a task.
          </Step>
        </div>
      </Card>

      <Card density="spacious" className="mb-5">
        <div className="flex items-center gap-2 mb-1">
          <Lock size={16} className="text-accent" />
          <h2 className="text-[16px] font-semibold text-text-0">Private agent</h2>
        </div>
        <p className="text-[13px] text-text-2 mb-3.5">
          A fully isolated swarm — only nodes holding your key can see or dispatch to each other.
          Invisible to the public network.
        </p>
        <div className="space-y-3.5">
          <Step n={1} title="Generate a shared swarm key">
            <Cmd code={`agentfm -mode genkey        # writes ./swarm.key`} />
          </Step>
          <Step n={2} title="Run a relay for your swarm">
            Locally or on a VPS. Copy the multiaddr it prints.
            <Cmd
              code={`agentfm -mode relay -port 4015 -swarmkey ./swarm.key
#  -> /ip4/<host>/tcp/4015/p2p/<relay-id>`}
            />
          </Step>
          <Step n={3} title="Run the worker on the swarm">
            Same worker command, plus the swarm key and the relay’s multiaddr.
            <Cmd
              code={`agentfm -mode worker \\
  -agentdir ./my-agent -image localhost/my-agent:latest \\
  -agent "My Agent" -capability "research" -model llama3.2 -maxtasks 4 \\
  -swarmkey ./swarm.key \\
  -bootstrap /ip4/<host>/tcp/4015/p2p/<relay-id>`}
            />
          </Step>
          <Step n={4} title="Add the project in the app">
            Open the project dropdown → <b>New project</b> → <b>Private</b>. Paste the{' '}
            <b>relay multiaddr</b> and the <b>swarm key</b> (the hex line inside{' '}
            <code>swarm.key</code>), then switch to it. Your private worker appears on the Radar.
          </Step>
        </div>
      </Card>

      <p className="text-[13px] text-text-2">
        Want the trust ledger to survive when every boss is offline? Run a{' '}
        <code className="text-text-1">agentfm -mode witness</code> alongside your relay — a
        lightweight replica a fresh boss can catch up from. Full reference in the{' '}
        <b>Developer</b> tab and the project docs.
      </p>
    </div>
  )
}
