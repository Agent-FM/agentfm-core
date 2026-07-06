import { HashRouter, Routes, Route, Navigate } from 'react-router-dom'
import { Shell } from './components/Shell'
import { BackendDownOverlay } from './components/BackendDownOverlay'
import { ErrorBoundary } from './components/ErrorBoundary'
import { DispatchDrawer } from './components/DispatchDrawer'
import { FeedbackModal } from './components/FeedbackModal'
import { ProjectSwitchingOverlay } from './components/projects/ProjectSwitchingOverlay'
import { CreateProjectWizard } from './components/projects/CreateProjectWizard'
import { SettingsSheet } from './components/SettingsSheet'
import { useBackend } from './hooks/useBackend'
import { useEventStream } from './hooks/useEventStream'
import Radar from './routes/Radar'
import Dashboard from './routes/Dashboard'
import Chat from './routes/Chat'
import PeerView from './routes/PeerView'
import Activity from './routes/Activity'
import Status from './routes/Status'
import Assets from './routes/Assets'
import Settings from './routes/Settings'
import Developer from './routes/Developer'
import { useWorkerHistory } from './hooks/useWorkerHistory'

export default function App() {
  const backend = useBackend()
  useEventStream()
  useWorkerHistory()
  const showOverlay = backend.consecutiveFailures >= 3

  return (
    <>
      <HashRouter>
        <ErrorBoundary>
          <Routes>
            <Route element={<Shell />}>
              <Route index element={<Navigate to="/radar" replace />} />
              <Route path="radar" element={<Radar />} />
              <Route path="dashboard" element={<Dashboard />} />
              <Route path="chat" element={<Chat />} />
              <Route path="chat/:sessionId" element={<Chat />} />
              <Route path="peer/:peerId" element={<PeerView />} />
              <Route path="activity" element={<Activity />} />
              <Route path="assets" element={<Assets />} />
              <Route path="status" element={<Status />} />
              <Route path="settings" element={<Settings />} />
              <Route path="developer" element={<Developer />} />
            </Route>
          </Routes>
        </ErrorBoundary>
      </HashRouter>
      <BackendDownOverlay show={showOverlay} />
      <DispatchDrawer />
      <FeedbackModal />
      <ProjectSwitchingOverlay />
      <CreateProjectWizard />
      <SettingsSheet />
    </>
  )
}
