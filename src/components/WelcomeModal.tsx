import { useEffect } from 'react'
import { useUIStore } from '../lib/store'

export function WelcomeModal() {
  const projects = useUIStore((s) => s.projects)
  const openWizard = useUIStore((s) => s.openCreateWizard)
  const isWizardOpen = useUIStore((s) => s.isCreateWizardOpen)

  useEffect(() => {
    if (projects.length === 0 && !isWizardOpen) {
      openWizard()
    }
  }, [projects.length, isWizardOpen, openWizard])

  return null
}
