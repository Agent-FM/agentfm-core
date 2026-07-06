import { motion } from 'framer-motion';
import { useCommentBody } from '../../lib/query';

interface Props {
  peerId: string;
  cid: string;
}

export function CommentBody({ peerId, cid }: Props) {
  const { data, isPending, error } = useCommentBody(peerId, cid);

  return (
    <motion.div
      initial={{ opacity: 0, y: 2 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.18 }}
      className="mt-2"
    >
      {isPending ? (
        <span className="text-xs text-text-2 italic">loading comment…</span>
      ) : error ? (
        <span className="text-xs text-rose-400">failed to load</span>
      ) : (
        <pre className="text-xs text-text-0 bg-bg-0 border border-border-0 rounded p-2.5 whitespace-pre-wrap font-sans">
          {data?.body}
        </pre>
      )}
    </motion.div>
  );
}
