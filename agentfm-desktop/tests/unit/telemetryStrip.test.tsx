import { describe, it, expect, beforeEach } from 'vitest'
import { render } from '@testing-library/react'
import { TelemetryStrip } from '../../src/components/peer/TelemetryStrip'
import { useMetricsStore } from '../../src/lib/metricsStore'

beforeEach(() => {
  useMetricsStore.getState().reset()
})

describe('TelemetryStrip', () => {
  it('shows the waiting placeholder when no buffer exists', () => {
    const { getByText } = render(<TelemetryStrip peerId="peerZ" />)
    expect(getByText(/Waiting for telemetry beacon/i)).toBeTruthy()
  })

  it('renders sparkline cells when the buffer has data', () => {
    useMetricsStore.getState().pushPeer('peerA', Date.now(), {
      cpu: 32,
      gpu: 68,
      ram: 4.2,
      queue: 2,
    })
    const { container, getByText } = render(<TelemetryStrip peerId="peerA" />)
    expect(container.querySelectorAll('canvas').length).toBeGreaterThanOrEqual(4)
    expect(getByText(/CPU/i)).toBeTruthy()
    expect(getByText(/GPU/i)).toBeTruthy()
    expect(getByText(/RAM/i)).toBeTruthy()
    expect(getByText(/QUEUE/i)).toBeTruthy()
  })

  it('shows offline notice when last tick is > 30s ago', () => {
    const longAgo = Date.now() - 60_000
    useMetricsStore.getState().pushPeer('peerB', longAgo, {
      cpu: 10, gpu: 20, ram: 1, queue: 0,
    })
    const { getByText } = render(<TelemetryStrip peerId="peerB" />)
    expect(getByText(/offline/i)).toBeTruthy()
  })
})
