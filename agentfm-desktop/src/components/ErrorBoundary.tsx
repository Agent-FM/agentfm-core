import { Component, ReactNode, ErrorInfo } from 'react'
import { toast } from 'sonner'

interface Props {
  children: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, error: null }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('[ErrorBoundary]', error, info)
    toast.error(`Unexpected error: ${error.message}`)
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="flex items-center justify-center h-full p-8 text-text-2">
          <div>
            <p className="font-semibold text-text-0 mb-1">Something went wrong. Reload?</p>
            <p className="text-xs font-mono text-text-2 mt-1">{this.state.error?.message}</p>
            <button
              className="mt-4 text-xs text-accent underline"
              onClick={() => location.reload()}
            >
              Reload window
            </button>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}
