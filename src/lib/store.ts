import { create } from 'zustand'
import { subscribeWithSelector } from 'zustand/middleware'
import type { Project } from '../types/project'
import {
  newProjectId,
  validateProjectInput,
  ProjectInput,
} from './projectStore'

export interface UIState {
  theme: 'dark' | 'light' | 'auto'
  accent: 'emerald' | 'violet' | 'rose'
  apiPort: number
  searchTerm: string
  filterTrustedOnly: boolean
  dispatchTarget: string | null
  isDispatchOpen: boolean
  isFeedbackOpen: boolean
  feedbackContext: { peerId: string; taskId: string } | null

  projects: Project[]
  activeProjectId: string | null
  isProjectSwitching: boolean
  isCreateWizardOpen: boolean
  isSettingsSheetOpen: boolean
  openSettingsSheet: () => void
  closeSettingsSheet: () => void

  setTheme: (t: UIState['theme']) => void
  setAccent: (a: UIState['accent']) => void
  setApiPort: (p: number) => void
  setSearchTerm: (s: string) => void
  setFilterTrustedOnly: (v: boolean) => void
  openDispatch: (peerId: string) => void
  closeDispatch: () => void
  openFeedback: (peerId: string, taskId: string) => void
  closeFeedback: () => void

  hydrateProjects: (
    projects: Project[],
    activeId: string | null,
  ) => void
  addProject: (input: ProjectInput) => Project
  updateProject: (id: string, patch: Partial<ProjectInput>) => void
  deleteProject: (id: string) => void
  setProjectSwitching: (v: boolean) => void
  switchProject: (id: string) => Promise<void>
  openCreateWizard: () => void
  closeCreateWizard: () => void

  activeProject: () => Project | undefined
}

export const useUIStore = create<UIState>()(
  subscribeWithSelector((set, get) => ({
    theme: 'dark',
    accent: 'emerald',
    apiPort: 8080,
    searchTerm: '',
    filterTrustedOnly: false,
    dispatchTarget: null,
    isDispatchOpen: false,
    isFeedbackOpen: false,
    feedbackContext: null,

    projects: [],
    activeProjectId: null,
    isProjectSwitching: false,
    isCreateWizardOpen: false,
    isSettingsSheetOpen: false,

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
    setSearchTerm: (searchTerm) => set({ searchTerm }),
    setFilterTrustedOnly: (filterTrustedOnly) => set({ filterTrustedOnly }),
    openDispatch: (dispatchTarget) => set({ isDispatchOpen: true, dispatchTarget }),
    closeDispatch: () => set({ isDispatchOpen: false, dispatchTarget: null }),
    openFeedback: (peerId, taskId) =>
      set({ isFeedbackOpen: true, feedbackContext: { peerId, taskId } }),
    closeFeedback: () => set({ isFeedbackOpen: false, feedbackContext: null }),

    hydrateProjects: (projects, activeId) => set({ projects, activeProjectId: activeId }),

    addProject: (input) => {
      const projects = get().projects
      validateProjectInput(projects, input)
      const project: Project = {
        id: newProjectId(),
        name: input.name.trim(),
        relayMultiaddr: input.relayMultiaddr,
        reputationFloor: input.reputationFloor ?? -0.5,
        createdAt: Date.now(),
      }
      const nextProjects = [...projects, project]
      set({ projects: nextProjects })
      window.api?.settings.set('projects', nextProjects).catch(() => {})
      return project
    },

    updateProject: (id, patch) => {
      const projects = get().projects
      const current = projects.find((p) => p.id === id)
      if (!current) return
      const merged: ProjectInput = {
        name: patch.name ?? current.name,
        relayMultiaddr:
          patch.relayMultiaddr === undefined ? current.relayMultiaddr : patch.relayMultiaddr,
        reputationFloor: patch.reputationFloor ?? current.reputationFloor,
      }
      validateProjectInput(projects, merged, id)
      const next = projects.map((p) =>
        p.id === id
          ? {
              ...p,
              ...merged,
              name: merged.name.trim(),
            }
          : p,
      )
      set({ projects: next })
      window.api?.settings.set('projects', next).catch(() => {})
    },

    deleteProject: (id) => {
      const projects = get().projects.filter((p) => p.id !== id)
      const activeId =
        get().activeProjectId === id ? (projects[0]?.id ?? null) : get().activeProjectId
      set({ projects, activeProjectId: activeId })
      window.api?.settings.set('projects', projects).catch(() => {})
      window.api?.settings.set('activeProjectId', activeId).catch(() => {})
      window.api?.settings.delete(`chat:sessions:${id}`).catch(() => {})
    },

    setProjectSwitching: (v) => set({ isProjectSwitching: v }),
    switchProject: async (id) => {
      const state = get()
      if (id === state.activeProjectId) return
      const project = state.projects.find((p) => p.id === id)
      if (!project) return
      set({ isProjectSwitching: true })
      try {
        await window.api?.settings.set('activeProjectId', id)
        set({ activeProjectId: id })
        await window.api?.backend.restart({
          apiPort: state.apiPort,
          reputationFloor: project.reputationFloor,
          relayMultiaddr: project.relayMultiaddr ?? undefined,
        })
      } catch (e) {
        console.warn('switchProject: backend restart failed', e)
      } finally {
        set({ isProjectSwitching: false })
      }
    },
    openCreateWizard: () => set({ isCreateWizardOpen: true }),
    closeCreateWizard: () => set({ isCreateWizardOpen: false }),
    openSettingsSheet: () => set({ isSettingsSheetOpen: true }),
    closeSettingsSheet: () => set({ isSettingsSheetOpen: false }),

    activeProject: () => {
      const { projects, activeProjectId } = get()
      return projects.find((p) => p.id === activeProjectId)
    },
  })),
)
