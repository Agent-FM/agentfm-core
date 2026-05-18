-- Schema v1 — verifiable agent mesh (v1.3).
--
-- Three append-only tables. UPDATE and DELETE on `entries` are rejected
-- at the trigger layer in addition to the Go API layer — the database
-- itself enforces append-only so an operator with `sqlite3` cannot
-- silently rewrite history.

CREATE TABLE IF NOT EXISTS entries (
    idx         INTEGER PRIMARY KEY AUTOINCREMENT,  -- 1-based, monotonically increasing
    hash        BLOB NOT NULL UNIQUE,               -- 32 bytes; RFC 6962 leaf hash
    prev_hash   BLOB NOT NULL,                      -- 32 bytes; predecessor's hash, or 32 zero bytes for first entry
    kind        INTEGER NOT NULL,                   -- 1 = Rating, 2 = Comment
    payload     BLOB NOT NULL,                      -- canonical pb.SignedEntry bytes (with Signature included)
    sig         BLOB NOT NULL,                      -- Ed25519 signature
    inserted_at INTEGER NOT NULL,                   -- unix nanoseconds local clock at append
    CHECK(length(hash)      = 32),
    CHECK(length(prev_hash) = 32),
    CHECK(kind IN (1, 2)),
    CHECK(inserted_at > 0)
);

CREATE INDEX IF NOT EXISTS entries_hash_idx ON entries(hash);

-- LogHead snapshots. We store every version we publish — including
-- intermediate states with partial witness sigs — keyed by tree_size.
-- The most recent row by signed_at is the "current" head.
CREATE TABLE IF NOT EXISTS heads (
    tree_size INTEGER PRIMARY KEY,
    root_hash BLOB NOT NULL,                -- 32 bytes
    signed_at INTEGER NOT NULL,             -- unix nanoseconds of issuance
    head_blob BLOB NOT NULL,                -- serialised pb.LogHead (with current witness_sigs)
    CHECK(length(root_hash) = 32),
    CHECK(signed_at > 0)
);

CREATE INDEX IF NOT EXISTS heads_signed_at_idx ON heads(signed_at);

-- Witness side-state: per-peer record of the last head this witness has
-- co-signed. Populated only on nodes running as witnesses (P2-1).
CREATE TABLE IF NOT EXISTS witness_state (
    peer_id    BLOB NOT NULL,
    tree_size  INTEGER NOT NULL,
    last_head  BLOB NOT NULL,
    PRIMARY KEY (peer_id)
);

-- Append-only enforcement: refuse UPDATE and DELETE on entries.
-- WriteHead overwrites are allowed (multiple witness sigs accumulate),
-- so heads has no such triggers.
CREATE TRIGGER IF NOT EXISTS entries_no_update
BEFORE UPDATE ON entries
BEGIN
    SELECT RAISE(ABORT, 'entries is append-only: UPDATE refused');
END;

CREATE TRIGGER IF NOT EXISTS entries_no_delete
BEFORE DELETE ON entries
BEGIN
    SELECT RAISE(ABORT, 'entries is append-only: DELETE refused');
END;
