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

// VerifyInclusionProof validates an InclusionProof against the
// LogHead it's anchored to. The check has three parts:
//
//  1. Ed25519 signature on the inner Rating/Comment is valid for its
//     RaterPeerID (same check VerifyEntry runs).
//  2. EntryHash of the entry matches the audit-path / position / root
//     in the proof's LogHead (RFC 6962 inclusion).
//  3. The peer-own signature on the LogHead is valid for head.PeerId.
//
// Returns (true, nil) only when all three pass. Witness signatures
// inside head.WitnessSigs are NOT validated here — callers that care
// about quorum should additionally call IsHeadValid(head, M).
func VerifyInclusionProof(proof *pb.InclusionProof) (bool, error) {
	if proof == nil {
		return false, errors.New("ledger: nil InclusionProof")
	}
	if proof.LogHead == nil {
		return false, errors.New("ledger: InclusionProof has no LogHead")
	}

	// InclusionProof carries the full SignedEntry wrapper (see Fix-6
	// audit-finding), so the verifier hashes the exact bytes the
	// prover signed — any future SignedEntry-level fields survive
	// the round-trip without silent verification failures.
	signed := proof.Entry
	if signed == nil || signed.GetBody() == nil {
		return false, errors.New("ledger: InclusionProof entry unset")
	}

	// 1. Entry-level signature.
	ok, err := VerifyEntry(signed)
	if err != nil {
		return false, fmt.Errorf("entry sig: %w", err)
	}
	if !ok {
		return false, nil
	}

	// 2. RFC 6962 inclusion.
	leafHash := EntryHash(signed)
	auditPath := make([][32]byte, len(proof.AuditPath))
	for i, bs := range proof.AuditPath {
		if len(bs) != 32 {
			return false, fmt.Errorf("ledger: audit_path[%d] not 32 bytes (got %d)", i, len(bs))
		}
		copy(auditPath[i][:], bs)
	}
	if len(proof.LogHead.RootHash) != 32 {
		return false, errors.New("ledger: log_head.root_hash not 32 bytes")
	}
	var root [32]byte
	copy(root[:], proof.LogHead.RootHash)
	if !merkle.VerifyInclusion(leafHash, proof.Position, proof.LogHead.TreeSize, root, auditPath) {
		return false, nil
	}

	// 3. Peer-own LogHead signature.
	headerPeer, err := peer.IDFromBytes(proof.LogHead.PeerId)
	if err != nil {
		return false, fmt.Errorf("log_head.peer_id: %w", err)
	}
	headerPub, err := headerPeer.ExtractPublicKey()
	if err != nil {
		return false, fmt.Errorf("log_head pubkey: %w", err)
	}
	canonical, err := pb.CanonicalLogHead(proof.LogHead)
	if err != nil {
		return false, fmt.Errorf("canonical log_head: %w", err)
	}
	digest := sha256.Sum256(canonical)
	headOK, err := headerPub.Verify(digest[:], proof.LogHead.Signature)
	if err != nil {
		return false, fmt.Errorf("verify log_head sig: %w", err)
	}
	return headOK, nil
}
