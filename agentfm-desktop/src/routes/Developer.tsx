import { useState } from 'react'
import { API_CATALOG } from '../lib/apiCatalog'
import { getApiBaseURL } from '../lib/api'
import { RoutePage } from '../components/primitives/RoutePage'
import { SectionLabel } from '../components/primitives/SectionLabel'
import { GettingStarted } from '../components/developer/GettingStarted'
import { EndpointList } from '../components/developer/EndpointList'
import { EndpointDetail } from '../components/developer/EndpointDetail'

export default function Developer() {
  const [selectedId, setSelectedId] = useState(API_CATALOG[0].id)
  const endpoint = API_CATALOG.find((e) => e.id === selectedId) ?? API_CATALOG[0]

  return (
    <RoutePage className="p-6">
      <SectionLabel>DEVELOPER</SectionLabel>
      <h1 className="text-3xl font-semibold mb-4">Developer API</h1>
      <GettingStarted baseURL={getApiBaseURL()} authEnabled={false} />
      <div className="flex gap-5 items-start">
        <EndpointList endpoints={API_CATALOG} selectedId={selectedId} onSelect={setSelectedId} />
        <EndpointDetail endpoint={endpoint} />
      </div>
    </RoutePage>
  )
}
