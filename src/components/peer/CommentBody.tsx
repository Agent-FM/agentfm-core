import { motion, AnimatePresence } from 'framer-motion';
import { Eye, EyeOff } from 'lucide-react';
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
      <button
        onClick={onToggle}
        title={expanded ? 'Hide comment' : 'Show comment'}
        aria-label={expanded ? 'Hide comment' : 'Show comment'}
        className="mt-1 inline-flex items-center gap-1 px-1.5 py-0.5 rounded
          text-[10px] text-accent border border-accent/25 hover:border-accent/55
          hover:bg-accent/10 transition-colors"
      >
        {expanded ? <EyeOff size={11} /> : <Eye size={11} />}
        <span>{expanded ? 'Hide' : 'Show'}</span>
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
