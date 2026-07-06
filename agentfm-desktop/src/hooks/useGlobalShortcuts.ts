import { useHotkeys } from 'react-hotkeys-hook';
import { useNavigate } from 'react-router-dom';
import { useUIStore } from '../lib/store';

export function useGlobalShortcuts() {
  const navigate = useNavigate();
  const closeDispatch = useUIStore((s) => s.closeDispatch);
  const closeFeedback = useUIStore((s) => s.closeFeedback);
  const setSearch = useUIStore((s) => s.setSearchTerm);

  useHotkeys('meta+1, ctrl+1', () => navigate('/radar'));
  useHotkeys('meta+2, ctrl+2', () => navigate('/chat'));
  useHotkeys('meta+3, ctrl+3', () => navigate('/activity'));
  useHotkeys('meta+4, ctrl+4', () => navigate('/status'));

  useHotkeys('mod+,', (e) => {
    e.preventDefault();
    useUIStore.getState().openSettingsSheet();
  }, []);

  // Cmd+K → focus radar search (or open command palette in v2)
  useHotkeys('meta+k, ctrl+k', (e) => {
    e.preventDefault();
    navigate('/radar');
    // Try to focus the search input via DOM query (cheap, no global focus ref needed)
    setTimeout(() => {
      const el = document.querySelector<HTMLInputElement>('input[placeholder*="Search by name"]');
      el?.focus();
    }, 50);
  });

  // Esc closes drawers/modals
  useHotkeys('esc', () => {
    closeDispatch();
    closeFeedback();
  });

  // Suppress unused variable warning for setSearch (reserved for future use)
  void setSearch;
}
