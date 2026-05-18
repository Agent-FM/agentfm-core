import { HashRouter, Routes, Route, Navigate } from 'react-router-dom'
import { Shell } from './components/Shell'
import { BackendDownOverlay } from './components/BackendDownOverlay'
import { ErrorBoundary } from './components/ErrorBoundary'
import { DispatchDrawer } from './components/DispatchDrawer'
import { FeedbackModal } from './components/FeedbackModal'
import { WelcomeModal } from './components/WelcomeModal'
import { ProjectSwitchingOverlay } from './components/projects/ProjectSwitchingOverlay'
import { ProjectSettingsSheet } from './components/projects/ProjectSettingsSheet'
import { CreateProjectWizard } from './components/projects/CreateProjectWizard'
import { useBackend } from './hooks/useBackend'
import { useEventStream } from './hooks/useEventStream'
import Settings from './routes/Settings'
import Radar from './routes/Radar'
import Chat from './routes/Chat'
import PeerView from './routes/PeerView'
import Activity from './routes/Activity'
import Status from './routes/Status'

export default function App() {
  const backend = useBackend()
  useEventStream()
  const showOverlay = backend.consecutiveFailures >= 3

  return (
    <>
      <HashRouter>
        <ErrorBoundary>
          <Routes>
            <Route element={<Shell />}>
              <Route index element={<Navigate to="/radar" replace />} />
              <Route path="radar" element={<Radar />} />
              <Route path="chat" element={<Chat />} />
              <Route path="chat/:sessionId" element={<Chat />} />
              <Route path="peer/:peerId" element={<PeerView />} />
              <Route path="activity" element={<Activity />} />
              <Route path="status" element={<Status />} />
              <Route path="settings" element={<Settings />} />
            </Route>
          </Routes>
        </ErrorBoundary>
      </HashRouter>
      <BackendDownOverlay show={showOverlay} />
      <DispatchDrawer />
      <FeedbackModal />
      <WelcomeModal />
      <ProjectSwitchingOverlay />
      <ProjectSettingsSheet />
      <CreateProjectWizard />
    </>
  )
}
