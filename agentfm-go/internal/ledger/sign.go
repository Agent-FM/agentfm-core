package ledger

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"agentfm/internal/ledger/merkle"
	pb "agentfm/internal/ledger/pb"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// SignEntry populates PrevHash and Signature on the inner Rating or
// Comment carried by entry, signing with key.
//
// The signature target is SHA-256(canonical_bytes), where canonical_bytes
// is the entry marshalled with the Signature field zeroed (see
// pb.CanonicalSignedEntry). The bare digest is what gets signed, NOT the
// canonical bytes themselves — this keeps Python verifiers in P4-4
// simple (compute the same SHA-256 over the same canonical bytes, verify
// against Ed25519) and avoids relying on byte-stable proto marshalling
// across language runtimes.
//
// entry MUST carry either a Rating or a Comment body; ErrUnsetBody is
// returned otherwise. The function mutates entry in place. Callers
// MUST ensure the inner RaterPeerID corresponds to key's PeerID;
// otherwise VerifyEntry will (correctly) reject the resulting entry.
func SignEntry(key crypto.PrivKey, entry *pb.SignedEntry, prevHash [32]byte) error {
	if key == nil {
		return errors.New("ledger: nil signing key")
	}
	if entry == nil {
		return errors.New("ledger: nil SignedEntry")
	}
	switch body := entry.GetBody().(type) {
	case *pb.SignedEntry_Rating:
		if body.Rating == nil {
			return ErrUnsetBody
		}
		body.Rating.PrevHash = prevHash[:]
		body.Rating.Signature = nil
		sig, err := signCanonical(key, entry)
		if err != nil {
			return err
		}
		body.Rating.Signature = sig
		return nil
	case *pb.SignedEntry_Comment:
		if body.Comment == nil {
			return ErrUnsetBody
		}
		body.Comment.PrevHash = prevHash[:]
		body.Comment.Signature = nil
		sig, err := signCanonical(key, entry)
		if err != nil {
			return err
		}
		body.Comment.Signature = sig
		return nil
	default:
		return ErrUnsetBody
	}
}

// VerifyEntry returns true iff the Signature on entry's inner body is
// a valid Ed25519 signature over SHA-256(canonical_bytes) from the
// peer identified by the inner body's RaterPeerID.
//
// The boolean / error split is deliberate:
//
//   - (false, nil)   means the signature did not verify — the entry
//     should be silently rejected.
//   - (false, err)   means we couldn't even attempt verification: the
//     entry was malformed, the peer id was unparseable, or the body
//     was missing. Callers may want to log these vs. the silent
//     mismatches above.
//   - (true, nil)    means the entry verified.
func VerifyEntry(entry *pb.SignedEntry) (bool, error) {
	if entry == nil {
		return false, errors.New("ledger: nil SignedEntry")
	}

	var raterPeerID, sig []byte
	switch body := entry.GetBody().(type) {
	case *pb.SignedEntry_Rating:
		if body.Rating == nil {
			return false, ErrUnsetBody
		}
		raterPeerID, sig = body.Rating.RaterPeerId, body.Rating.Signature
	case *pb.SignedEntry_Comment:
		if body.Comment == nil {
			return false, ErrUnsetBody
		}
		raterPeerID, sig = body.Comment.RaterPeerId, body.Comment.Signature
	default:
		return false, ErrUnsetBody
	}

	pid, err := peer.IDFromBytes(raterPeerID)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrInvalidRaterPeerID, err)
	}
	pub, err := pid.ExtractPublicKey()
	if err != nil {
		// For Ed25519 keys (the only kind worker_identity.key produces)
		// ExtractPublicKey always succeeds. Surfaces here when a peer
		// uses an unsupported key type or a hashed-PeerID it did not
		// expand.
		return false, fmt.Errorf("%w: %v", ErrInvalidRaterPeerID, err)
	}

	digest, err := signingDigest(entry)
	if err != nil {
		return false, err
	}
	return pub.Verify(digest[:], sig)
}

// EntryHash returns merkle.HashLeaf(canonical_bytes(entry)) — i.e. the
// RFC 6962 leaf hash that this entry occupies in the Merkle tree.
//
// This is also the value that subsequent entries place in their
// prev_hash field, so the chain and the tree share a single canonical
// identifier per entry: a verifier can use it to walk the prev_hash
// chain or to look up a Merkle inclusion proof, without converting
// between two flavours of "the entry's hash."
//
// On malformed input (nil entry, unset body, marshal error) returns the
// zero array. Callers should treat that as "no valid hash exists" and
// reject the entry; we don't bubble an error to keep the helper
// ergonomic for log walks where the entry is already known to be
// well-formed.
func EntryHash(entry *pb.SignedEntry) [32]byte {
	canonical, err := pb.CanonicalSignedEntry(entry)
	if err != nil {
		return [32]byte{}
	}
	return merkle.HashLeaf(canonical)
}

// signCanonical returns an Ed25519 signature over SHA-256(canonical_bytes(entry)).
// Helper shared by both SignEntry oneof branches.
func signCanonical(key crypto.PrivKey, entry *pb.SignedEntry) ([]byte, error) {
	digest, err := signingDigest(entry)
	if err != nil {
		return nil, err
	}
	sig, err := key.Sign(digest[:])
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}
	return sig, nil
}

// signingDigest computes SHA-256(CanonicalSignedEntry(entry)).
// Used by both signing and verification — they MUST agree on the byte
// stream that gets fed into SHA-256, otherwise verification fails
// silently in confusing ways.
func signingDigest(entry *pb.SignedEntry) ([32]byte, error) {
	canonical, err := pb.CanonicalSignedEntry(entry)
	if err != nil {
		return [32]byte{}, fmt.Errorf("canonical bytes: %w", err)
	}
	return sha256.Sum256(canonical), nil
}
