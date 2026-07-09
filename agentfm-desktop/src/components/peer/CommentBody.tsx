import { motion } from 'framer-motion';
import { useCommentBody } from '../../lib/query';
import { SkeletonBox } from '../primitives/Skeleton';

interface Props {
  peerId: string;
  cid: string;
}

export function CommentBody({ peerId, cid }: Props) {
  const { data, isPending, error } = useCommentBody(peerId, cid);

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.15, ease: 'easeOut' }}
      className="mt-1.5"
    >
      {isPending ? (
        <div className="glass-inset rounded-ctl p-2 space-y-1.5">
          <SkeletonBox className="h-2.5 w-full" />
          <SkeletonBox className="h-2.5 w-4/5" />
        </div>
      ) : error ? (
        <span className="text-xs text-bad">Failed to load comment</span>
      ) : (
        <pre className="text-xs text-text-0 glass-inset rounded-ctl p-2 whitespace-pre-wrap font-sans">
          {data?.body}
        </pre>
      )}
    </motion.div>
  );
}
