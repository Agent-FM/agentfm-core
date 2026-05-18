import { useEffect, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Button } from '../primitives/Button';

interface Props { isOpen: boolean; onClose: () => void; }

export function LogsModal({ isOpen, onClose }: Props) {
  const [lines, setLines] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);

  async function refresh() {
    setLoading(true);
    try {
      const ls = await window.api.backend.logs(500);
      setLines(ls);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    if (isOpen) refresh();
  }, [isOpen]);

  useEffect(() => {
    function handler(e: KeyboardEvent) { if (e.key === 'Escape' && isOpen) onClose(); }
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [isOpen, onClose]);

  return (
    <AnimatePresence>
      {isOpen && (
        <motion.div
          initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
          className="fixed inset-0 bg-black/70 backdrop-blur-sm z-[60] flex items-center justify-center p-8"
          onClick={onClose}
        >
          <motion.div
            initial={{ scale: 0.97, opacity: 0 }} animate={{ scale: 1, opacity: 1 }} exit={{ scale: 0.97, opacity: 0 }}
            transition={{ type: 'spring', stiffness: 350, damping: 28 }}
            onClick={(e) => e.stopPropagation()}
            className="bg-bg-1 border border-border-0 rounded-xl shadow-2xl w-full max-w-4xl max-h-[80vh] flex flex-col"
          >
            <div className="flex justify-between items-center p-4 border-b border-border-0">
              <h3 className="text-base font-semibold text-text-0">Backend logs</h3>
              <div className="flex gap-2 items-center">
                <Button onClick={refresh} disabled={loading}>{loading ? 'Refreshing…' : 'Refresh'}</Button>
                <button onClick={onClose} className="text-text-2 hover:text-text-0 text-lg ml-2">✕</button>
              </div>
            </div>
            <pre className="flex-1 overflow-auto p-4 text-[11px] font-mono text-text-2 leading-relaxed whitespace-pre-wrap">
              {lines.length === 0 ? '(no log output)' : lines.join('')}
            </pre>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}
