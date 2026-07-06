interface Props {
  output: string;
  streaming: boolean;
}

export function StreamingView({ output, streaming }: Props) {
  return (
    <div className="relative bg-bg-0 border border-border-0 rounded-xl p-4 max-h-[400px] overflow-auto font-mono text-xs">
      {streaming && (
        <span className="absolute top-0 left-0 right-0 h-[2px] bg-gradient-to-r from-transparent via-accent to-transparent animate-shimmer" />
      )}
      <pre className="whitespace-pre-wrap text-text-1">{output}</pre>
      {streaming && <span className="inline-block w-[3px] h-3.5 bg-accent ml-0.5 align-middle animate-pulse-cyan" />}
    </div>
  );
}
