import { motion, AnimatePresence } from 'framer-motion';
import { useCommentBody } from '../../lib/query';

interface Props {
  peerId: string;
  cid: string;
  expanded: boolean;
  onToggle: () => void;
}

export function CommentBodyExpander({ peerId, cid, expanded, onToggle }: Props) {
  const { data, isPending, error } = useCommentBody(peerId, expanded ? cid : undefined);

  return (
    <>
      <button onClick={onToggle} className="text-[10px] text-accent hover:underline mt-1">
        {expanded ? '▼ Hide body' : '▶ Show body'}
      </button>
      <AnimatePresence>
        {expanded && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
            transition={{ duration: 0.18 }}
            className="overflow-hidden mt-2"
          >
            {isPending ? (
              <span className="text-xs text-text-2 italic">loading body…</span>
            ) : error ? (
              <span className="text-xs text-rose-400">failed to load</span>
            ) : (
              <pre className="text-xs text-text-0 bg-bg-0 border border-border-0 rounded p-2.5 whitespace-pre-wrap font-sans">
                {data?.body}
              </pre>
            )}
          </motion.div>
        )}
      </AnimatePresence>
    </>
  );
}
