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

// ErrInvalidRaterPeerID is returned by VerifyEntry when the RaterPeerID
// field cannot be parsed as a libp2p PeerID — typically because the
// entry was malformed on the wire or the bytes do not encode a known
// key type. Callers should treat this as "entry rejected" and never
// allow it to enter their local inbox.
var ErrInvalidRaterPeerID = errors.New("ledger: invalid rater peer id")

// ErrUnsetBody is returned by SignEntry / VerifyEntry when the
// SignedEntry oneof is empty. Programming error in the caller.
var ErrUnsetBody = errors.New("ledger: SignedEntry has no body set")
