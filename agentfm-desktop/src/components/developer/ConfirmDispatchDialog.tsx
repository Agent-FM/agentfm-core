import type { SideEffect } from '../../lib/apiCatalog'
import { Button } from '../primitives/Button'
import { GradientButton } from '../primitives/GradientButton'

interface Props {
  sideEffect: Exclude<SideEffect, 'none'>
  onConfirm: () => void
  onCancel: () => void
}

const COPY: Record<Exclude<SideEffect, 'none'>, string> = {
  dispatch: 'This will run a real task in a container on a worker (consumes compute).',
  signed: 'This will be cryptographically signed by your Boss and appended to the reputation ledger.',
}

export function ConfirmDispatchDialog({ sideEffect, onConfirm, onCancel }: Props) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onCancel}>
      <div className="glass-strong rounded-2xl p-5 max-w-md mx-4" onClick={(e) => e.stopPropagation()}>
        <h3 className="text-lg font-semibold mb-2">Confirm request</h3>
        <p className="text-sm text-text-1 mb-4">{COPY[sideEffect]}</p>
        <div className="flex justify-end gap-2">
          <Button onClick={onCancel}>Cancel</Button>
          <GradientButton onClick={onConfirm}>Send anyway</GradientButton>
        </div>
      </div>
    </div>
  )
}
