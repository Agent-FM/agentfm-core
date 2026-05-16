package ledger

import "errors"

// ErrNotImplemented is returned by every public Ledger method until the
// corresponding P1-* / P2-* / P3-* ticket lands. Callers SHOULD treat
// it as a programming error in production builds — if the ledger is
// unwired, the boss/worker bootstrap should never have constructed
// one. Tests use errors.Is(err, ErrNotImplemented) to assert that
// stubbed methods have not been accidentally wired without an update
// to the test suite.
var ErrNotImplemented = errors.New("ledger: not implemented (waiting on P1-* / P2-* implementation)")
