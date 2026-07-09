import { motion, AnimatePresence } from 'framer-motion';
import { useState, useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useUIStore } from '../lib/store';
import { Button } from './primitives/Button';
import { toast } from 'sonner';
import { api, ApiError } from '../lib/api';
import { qk } from '../lib/query';
import { X, Send } from 'lucide-react'
import { StarRow } from './primitives/StarRow'
import { starsFromScore } from '../lib/stars'

export function FeedbackModal() {
  const isOpen = useUIStore((s) => s.isFeedbackOpen);
  const ctx = useUIStore((s) => s.feedbackContext);
  const close = useUIStore((s) => s.closeFeedback);
  const qc = useQueryClient();

  const [text, setText] = useState('');
  const [rating, setRating] = useState(0);
  const [hasRating, setHasRating] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!isOpen) {
      setText('');
      setRating(0);
      setHasRating(false);
      setSubmitting(false);
    }
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen) return;
    function handler(e: KeyboardEvent) {
      if (e.key === 'Escape') close();
      if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
        if (text.trim() && ctx && !submitting) submit();
      }
    }
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, text, rating, hasRating, ctx, submitting]);

  async function submit() {
    if (!text.trim() || !ctx || submitting) return;
    setSubmitting(true);
    try {
      // Send the rating as a discrete field so the boss appends a paired
      // Rating ledger entry. Previously the rating was string-concatenated
      // into the comment body, visible but cosmetic; it never flowed into
      // EigenTrust or appeared under the PeerView Ratings tab.
      const res = await api.submitSelfComment(ctx.peerId, {
        text: text.trim(),
        language: 'en',
        rating: hasRating ? rating : undefined,
      });
      await Promise.all([
        qc.invalidateQueries({ queryKey: qk.peer(ctx.peerId) }),
        qc.invalidateQueries({
          predicate: (q) =>
            Array.isArray(q.queryKey) &&
            q.queryKey[0] === 'peer-log' &&
            q.queryKey[1] === ctx.peerId,
        }),
      ]);
      toast.success(`Feedback signed and gossiped, cid ${res.cid.slice(0, 10)}…`);
      close();
    } catch (err) {
      const msg = err instanceof ApiError ? `${err.status}: ${err.message}` : String(err);
      toast.error('Feedback failed: ' + msg);
      setSubmitting(false);
    }
  }

  if (!ctx) return null;

  return (
    <AnimatePresence>
      {isOpen && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.15 }}
          className="fixed inset-0 bg-bg-0/75 z-[60] flex items-center justify-center"
          onClick={close}
        >
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.15, ease: 'easeOut' }}
            onClick={(e) => e.stopPropagation()}
            className="w-[480px] glass-strong rounded-sheet shadow-float p-4"
          >
            <div className="flex justify-between items-start mb-3">
              <div>
                <h3 className="text-lg font-semibold text-text-0">Leave feedback</h3>
                <div className="flex items-center gap-2 text-2xs font-mono text-text-2 mt-0.5 tabular-nums">
                  <span>peer {ctx.peerId.slice(0, 10)}…</span>
                  <span className="h-2.5 w-px bg-border-1" />
                  <span>task {ctx.taskId}</span>
                </div>
              </div>
              <button onClick={close} aria-label="Close feedback" className="text-text-2 hover:text-text-0 transition-colors duration-150">
                <X size={16} strokeWidth={1.5} />
              </button>
            </div>

            <label className="text-2xs font-medium text-text-2 block mb-1.5">
              Your comment
            </label>
            <textarea
              autoFocus
              value={text}
              onChange={(e) => setText(e.target.value)}
              aria-label="Your comment"
              placeholder="What was the outcome?"
              className="w-full min-h-[80px] glass-inset rounded-ctl p-3 text-sm text-text-0 outline-none focus:border-accent/60 resize-y"
            />

            <label className="text-2xs font-medium text-text-2 block mt-3 mb-1.5">
              Rating <span className="text-text-3">(optional)</span>
            </label>
            <div className="mb-2">
              <StarRow value={hasRating ? starsFromScore(rating) : 0} size={18} />
            </div>
            <div className="flex items-center gap-3">
              <input
                type="range"
                min={-1}
                max={1}
                step={0.05}
                value={rating}
                aria-label="Rating from -1 (bad) to +1 (good)"
                onChange={(e) => {
                  setRating(Number(e.target.value));
                  setHasRating(true);
                }}
                className="flex-1 accent-accent"
              />
              <span
                className={`font-mono text-xs w-12 text-right tabular-nums ${
                  hasRating
                    ? rating > 0
                      ? 'text-ok'
                      : rating < 0
                        ? 'text-bad'
                        : 'text-text-1'
                    : 'text-text-3'
                }`}
              >
                {hasRating ? (rating >= 0 ? '+' : '') + rating.toFixed(2) : 'n/a'}
              </span>
            </div>

            <div className="flex gap-2 justify-end mt-4">
              <Button onClick={close}>Skip</Button>
              <Button variant="primary" onClick={submit} disabled={!text.trim() || submitting}>
                <Send size={12} />
                <span>{submitting ? 'Signing…' : 'Sign & send'}</span>
              </Button>
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
