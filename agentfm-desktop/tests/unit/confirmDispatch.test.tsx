import { describe, it, expect, vi } from 'vitest'
import { render, fireEvent } from '@testing-library/react'
import { ConfirmDispatchDialog } from '../../src/components/developer/ConfirmDispatchDialog'

describe('ConfirmDispatchDialog', () => {
  it('names the side effect and fires onConfirm', () => {
    const onConfirm = vi.fn()
    const onCancel = vi.fn()
    const { getByText } = render(
      <ConfirmDispatchDialog sideEffect="dispatch" onConfirm={onConfirm} onCancel={onCancel} />,
    )
    expect(getByText(/run a real task/i)).toBeTruthy()
    fireEvent.click(getByText('Send anyway'))
    expect(onConfirm).toHaveBeenCalled()
  })

  it('explains signing for signed endpoints', () => {
    const { getByText } = render(
      <ConfirmDispatchDialog sideEffect="signed" onConfirm={() => {}} onCancel={() => {}} />,
    )
    expect(getByText(/cryptographically signed/i)).toBeTruthy()
  })

  it('calls onCancel when Escape is pressed', () => {
    const onCancel = vi.fn()
    render(<ConfirmDispatchDialog sideEffect="dispatch" onConfirm={() => {}} onCancel={onCancel} />)
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(onCancel).toHaveBeenCalled()
  })
})
