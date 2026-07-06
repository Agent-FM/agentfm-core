import { Component, ReactNode } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'

interface State {
  error: Error | null
}

interface Props {
  children: ReactNode
}

class Boundary extends Component<Props & { onReset: () => void; onHome: () => void; pathname: string }, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidUpdate(prevProps: Props & { pathname: string }) {
    if (prevProps.pathname !== this.props.pathname && this.state.error) {
      this.setState({ error: null })
    }
  }

  render() {
    if (this.state.error) {
      return (
        <div className="p-8 max-w-xl">
          <div className="bg-rose-950/40 border border-rose-900/60 rounded-xl p-5">
            <div className="text-rose-300 font-semibold text-sm mb-1">This view hit a snag</div>
            <div className="text-text-1 text-sm mb-3">
              {this.state.error.message || 'Unknown error'}
            </div>
            <div className="flex gap-2">
              <button
                onClick={() => {
                  this.setState({ error: null })
                  this.props.onReset()
                }}
                className="bg-bg-2 border border-border-0 rounded-md px-3 py-1.5 text-xs text-text-1 hover:text-text-0"
              >
                Retry
              </button>
              <button
                onClick={this.props.onHome}
                className="bg-accent text-accent-fg rounded-md px-3 py-1.5 text-xs font-medium"
              >
                Go to Radar
              </button>
            </div>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}

export function RouteErrorBoundary({ children }: { children: ReactNode }) {
  const navigate = useNavigate()
  const location = useLocation()
  return (
    <Boundary
      onReset={() => navigate(location.pathname, { replace: true })}
      onHome={() => navigate('/radar')}
      pathname={location.pathname}
    >
      {children}
    </Boundary>
  )
}
