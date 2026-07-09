interface Props {
  output: string;
  streaming: boolean;
}

export function StreamingView({ output, streaming }: Props) {
  return (
    <div className="relative console-well mono-console rounded-ctl p-3 max-h-[400px] overflow-auto font-mono text-xs">
      <pre className="whitespace-pre-wrap">{output}</pre>
      {streaming && <span className="inline-block w-[3px] h-3.5 bg-accent ml-0.5 align-middle animate-pulse-cyan" />}
    </div>
  );
}
