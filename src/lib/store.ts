import { create } from 'zustand'
import { subscribeWithSelector } from 'zustand/middleware'

export interface UIState {
  theme: 'dark' | 'light' | 'auto'
  accent: 'emerald' | 'violet' | 'rose'
  apiPort: number
  reputationFloor: number
  relayMultiaddr: string | null
  searchTerm: string
  filterTrustedOnly: boolean
  dispatchTarget: string | null
  isDispatchOpen: boolean
  isFeedbackOpen: boolean
  feedbackContext: { peerId: string; taskId: string } | null

  setTheme: (t: UIState['theme']) => void
  setAccent: (a: UIState['accent']) => void
  setApiPort: (p: number) => void
  setReputationFloor: (f: number) => void
  setRelayMultiaddr: (m: string | null) => void
  setSearchTerm: (s: string) => void
  setFilterTrustedOnly: (v: boolean) => void
  openDispatch: (peerId: string) => void
  closeDispatch: () => void
  openFeedback: (peerId: string, taskId: string) => void
  closeFeedback: () => void
}

export const useUIStore = create<UIState>()(
  subscribeWithSelector((set) => ({
    theme: 'dark',
    accent: 'emerald',
    apiPort: 8080,
    reputationFloor: -0.5,
    relayMultiaddr: null,
    searchTerm: '',
    filterTrustedOnly: false,
    dispatchTarget: null,
    isDispatchOpen: false,
    isFeedbackOpen: false,
    feedbackContext: null,

    setTheme: (theme) => {
      set({ theme })
      document.documentElement.setAttribute('data-theme', theme)
      window.api?.settings.set('theme', theme).catch(() => {})
    },
    setAccent: (accent) => {
      set({ accent })
      document.documentElement.setAttribute('data-accent', accent)
      window.api?.settings.set('accent', accent).catch(() => {})
    },
    setApiPort: (apiPort) => set({ apiPort }),
    setReputationFloor: (reputationFloor) => set({ reputationFloor }),
    setRelayMultiaddr: (relayMultiaddr) => set({ relayMultiaddr }),
    setSearchTerm: (searchTerm) => set({ searchTerm }),
    setFilterTrustedOnly: (filterTrustedOnly) => set({ filterTrustedOnly }),
    openDispatch: (dispatchTarget) => set({ isDispatchOpen: true, dispatchTarget }),
    closeDispatch: () => set({ isDispatchOpen: false, dispatchTarget: null }),
    openFeedback: (peerId, taskId) =>
      set({ isFeedbackOpen: true, feedbackContext: { peerId, taskId } }),
    closeFeedback: () => set({ isFeedbackOpen: false, feedbackContext: null }),
  })),
)
