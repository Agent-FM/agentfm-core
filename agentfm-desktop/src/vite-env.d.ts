/// <reference types="vite/client" />

// Electron custom CSS property for draggable window regions (frameless title
// bar). Not part of the standard React CSSProperties, so augment it here.
import 'react'
declare module 'react' {
  interface CSSProperties {
    WebkitAppRegion?: 'drag' | 'no-drag'
  }
}
