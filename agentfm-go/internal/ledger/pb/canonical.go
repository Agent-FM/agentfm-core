// Canonical serialisation for ledger envelopes.
//
// SECURITY: every signature in the verifiable-mesh design is computed as
//
//	sig = Ed25519.Sign(privkey, SHA-256(CanonicalX(msg)))
//
// CanonicalX zeroes the fields that are mutated after signing (Signature,
// WitnessSigs, RekorAnchor) so verifiers reconstruct identical bytes from
// any received copy. Never feed raw proto.Marshal output into a signer —
// the included Signature field would create a self-referential target and
// any added WitnessSig would invalidate previously-collected signatures.
package pb

import (
	"errors"

	"google.golang.org/protobuf/proto"
)

// ErrUnknownEnvelope is returned by CanonicalSignedEntry when the inner
// oneof body has not been set (typically a programming bug — the caller
// constructed a SignedEntry without assigning Body).
var ErrUnknownEnvelope = errors.New("pb.canonical: SignedEntry has no body set")

// detMarshal is the single source of marshal-options for canonical bytes.
// Deterministic: true gives stable byte output for a given message within
// the Go process; combined with our field-zeroing above it is what makes
// CanonicalX("the same logical message") produce the same bytes on every
// call.
var detMarshal = proto.MarshalOptions{Deterministic: true}

// CanonicalRating returns the bytes to be hashed before signing a Rating.
// The Signature field is stripped.
func CanonicalRating(r *Rating) ([]byte, error) {
	if r == nil {
		return nil, errors.New("pb.canonical: nil Rating")
	}
	clone, _ := proto.Clone(r).(*Rating)
	clone.Signature = nil
	return detMarshal.Marshal(clone)
}

// CanonicalComment returns the bytes to be hashed before signing a Comment.
// The Signature field is stripped.
func CanonicalComment(c *Comment) ([]byte, error) {
	if c == nil {
		return nil, errors.New("pb.canonical: nil Comment")
	}
	clone, _ := proto.Clone(c).(*Comment)
	clone.Signature = nil
	return detMarshal.Marshal(clone)
}

// CanonicalLogHead returns the bytes to be hashed for either the peer's
// own LogHead signature or a witness co-signature. Signature, WitnessSigs,
// and RekorAnchor are all stripped — all participants must agree on the
// same bytes regardless of which signatures have been collected so far.
func CanonicalLogHead(h *LogHead) ([]byte, error) {
	if h == nil {
		return nil, errors.New("pb.canonical: nil LogHead")
	}
	clone, _ := proto.Clone(h).(*LogHead)
	clone.Signature = nil
	clone.WitnessSigs = nil
	clone.RekorAnchor = ""
	return detMarshal.Marshal(clone)
}

// CanonicalEquivocationAlert returns the bytes to be hashed before the
// witness signs an alert. The witness_signature field is stripped.
func CanonicalEquivocationAlert(a *EquivocationAlert) ([]byte, error) {
	if a == nil {
		return nil, errors.New("pb.canonical: nil EquivocationAlert")
	}
	clone, _ := proto.Clone(a).(*EquivocationAlert)
	clone.WitnessSignature = nil
	return detMarshal.Marshal(clone)
}

// CanonicalSignedEntry dispatches to the correct per-envelope helper
// based on the SignedEntry oneof body. This is the entry point most
// ledger code should call when it needs to sign or verify an entry
// without caring whether it is a Rating or a Comment.
func CanonicalSignedEntry(e *SignedEntry) ([]byte, error) {
	if e == nil {
		return nil, errors.New("pb.canonical: nil SignedEntry")
	}
	switch body := e.GetBody().(type) {
	case *SignedEntry_Rating:
		return CanonicalRating(body.Rating)
	case *SignedEntry_Comment:
		return CanonicalComment(body.Comment)
	case nil:
		return nil, ErrUnknownEnvelope
	default:
		return nil, ErrUnknownEnvelope
	}
}
