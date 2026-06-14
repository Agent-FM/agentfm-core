import { motion, AnimatePresence } from 'framer-motion';
import { useState, useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useUIStore } from '../lib/store';
import { Button } from './primitives/Button';
import { toast } from 'sonner';
import { api, ApiError } from '../lib/api';
import { qk } from '../lib/query';
import { X, Send } from 'lucide-react'

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
      // into the comment body — visible but cosmetic; it never flowed into
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
      toast.success(`Feedback signed and gossipped 💌 cid ${res.cid.slice(0, 10)}…`);
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
          className="fixed inset-0 bg-black/75 backdrop-blur-sm z-[60] flex items-center justify-center"
          onClick={close}
        >
          <motion.div
            initial={{ scale: 0.95, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            exit={{ scale: 0.95, opacity: 0 }}
            transition={{ type: 'spring', stiffness: 380, damping: 28 }}
            onClick={(e) => e.stopPropagation()}
            className="w-[480px] bg-bg-1 border border-border-0 rounded-2xl p-7 shadow-2xl"
          >
            <div className="flex justify-between items-start mb-4">
              <div>
                <h3 className="text-xl font-semibold tracking-tight text-text-0">Leave feedback</h3>
                <div className="text-[11px] font-mono text-text-2 mt-1">
                  about peer {ctx.peerId.slice(0, 10)}… · task {ctx.taskId}
                </div>
              </div>
              <button onClick={close} className="text-text-2 hover:text-text-0 text-lg">
                <X size={18} />
              </button>
            </div>

            <label className="text-[11px] uppercase tracking-wider text-text-2 block mb-1.5">
              Your comment
            </label>
            <textarea
              autoFocus
              value={text}
              onChange={(e) => setText(e.target.value)}
              placeholder="What was the outcome?"
              className="w-full min-h-[80px] bg-bg-0 border border-border-0 rounded-md p-3 text-sm text-text-0 outline-none focus:border-accent resize-y"
            />

            <label className="text-[11px] uppercase tracking-wider text-text-2 block mt-4 mb-1.5">
              Rating{' '}
              <span className="text-text-3 normal-case">(optional)</span>
            </label>
            <div className="flex items-center gap-3">
              <input
                type="range"
                min={-1}
                max={1}
                step={0.05}
                value={rating}
                onChange={(e) => {
                  setRating(Number(e.target.value));
                  setHasRating(true);
                }}
                className="flex-1 accent-accent"
              />
              <span
                className={`font-mono text-sm w-12 text-right ${
                  hasRating
                    ? rating > 0
                      ? 'text-emerald-400'
                      : rating < 0
                        ? 'text-rose-400'
                        : 'text-text-1'
                    : 'text-text-3'
                }`}
              >
                {hasRating ? (rating >= 0 ? '+' : '') + rating.toFixed(2) : '—'}
              </span>
            </div>

            <div className="text-[10px] text-text-2 mt-3">
              Signed by your boss identity · gossiped on{' '}
              <code className="font-mono">agentfm-feedback-v1</code> · permanent
            </div>

            <div className="flex gap-2 justify-end mt-5">
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
