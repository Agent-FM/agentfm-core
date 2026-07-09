import { useState } from 'react'
import { API_CATALOG } from '../lib/apiCatalog'
import { getApiBaseURL } from '../lib/api'
import { GettingStarted } from '../components/developer/GettingStarted'
import { EndpointList } from '../components/developer/EndpointList'
import { EndpointDetail } from '../components/developer/EndpointDetail'

export default function Developer() {
  const [selectedId, setSelectedId] = useState('overview')
  const endpoint = API_CATALOG.find((e) => e.id === selectedId)

  return (
    <div className="flex h-full min-h-0">
      <EndpointList endpoints={API_CATALOG} selectedId={selectedId} onSelect={setSelectedId} />
      <div className="flex-1 min-w-0 h-full overflow-auto bg-editor">
        <div className="p-4 max-w-3xl">
          {endpoint ? (
            <EndpointDetail endpoint={endpoint} />
          ) : (
            <>
              <h1 className="text-lg font-semibold text-text-0 mb-3">Developer API</h1>
              <GettingStarted baseURL={getApiBaseURL()} authEnabled={false} />
            </>
          )}
        </div>
      </div>
    </div>
  )
}
