import { useEffect, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { X, RefreshCw } from 'lucide-react';
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
          className="fixed inset-0 bg-black/70 backdrop-blur-sm z-[60] flex items-end justify-center"
          onClick={onClose}
        >
          <motion.div
            initial={{ y: '100%' }} animate={{ y: 0 }} exit={{ y: '100%' }}
            transition={{ type: 'spring', stiffness: 350, damping: 32 }}
            onClick={(e) => e.stopPropagation()}
            className="fixed bottom-0 left-0 right-0 z-[70] bg-bg-1 border-t border-border-0 rounded-t-2xl overflow-hidden flex flex-col"
            style={{ height: '60vh' }}
          >
            <div className="w-12 h-1 rounded-full bg-text-3 mx-auto mt-2 mb-1" />
            <div className="flex justify-between items-center p-4 border-b border-border-0">
              <h3 className="text-base font-semibold text-text-0">Backend logs</h3>
              <div className="flex gap-2 items-center">
                <Button onClick={refresh} disabled={loading}>
                  <RefreshCw size={12} className={loading ? 'animate-spin' : ''} />
                </Button>
                <button onClick={onClose} className="text-text-2 hover:text-text-0 ml-2">
                  <X size={18} />
                </button>
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
