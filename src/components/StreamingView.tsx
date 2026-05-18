interface Props {
  output: string;
  streaming: boolean;
}

export function StreamingView({ output, streaming }: Props) {
  return (
    <pre className="bg-bg-0 border border-border-0 rounded-md p-3.5 font-mono text-xs text-text-1 leading-relaxed min-h-[120px] max-h-[400px] overflow-auto whitespace-pre-wrap break-words">
      {output}
      {streaming && (
        <span className="inline-block w-2 h-3.5 bg-accent ml-0.5 animate-blink align-middle" />
      )}
    </pre>
  );
}
