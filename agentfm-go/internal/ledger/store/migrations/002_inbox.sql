-- Schema v2 — inbox tables (P1-5).
--
-- The inbox holds entries received from OTHER peers over gossip.
-- It is structurally separate from the `entries` table, which holds only
-- the local peer's own outgoing log:
--
--   * `entries`           → this peer's own append-only signed log
--                           (idx is THIS peer's autoincrement)
--   * `inbox_entries`     → entries from other peers, accepted into the
--                           rater's known chain. Keyed by (peer_id, hash);
--                           NO autoincrement — the rater's own log order
--                           is preserved by their prev_hash chain.
--   * `inbox_orphans`     → entries received before their parent. Bounded
--                           in size; promoted to inbox_entries when the
--                           parent arrives.
--   * `inbox_known_chain_head`
--                         → per-peer "the latest hash we know from this
--                           peer"; the chain-extension check reads this.
--
-- These tables are NOT append-only at the database level: orphan
-- entries get moved/deleted, and the chain head updates over time.
-- Append-only enforcement still applies to `entries` only.

CREATE TABLE IF NOT EXISTS inbox_entries (
    peer_id     BLOB    NOT NULL,    -- libp2p peer id of the rater
    hash        BLOB    NOT NULL,    -- 32-byte RFC 6962 leaf hash
    prev_hash   BLOB    NOT NULL,    -- 32-byte hash of predecessor in rater's chain
    payload     BLOB    NOT NULL,    -- full pb.SignedEntry proto (signature included)
    received_at INTEGER NOT NULL,    -- unix nanoseconds when this node accepted
    PRIMARY KEY (peer_id, hash),
    CHECK(length(hash) = 32),
    CHECK(length(prev_hash) = 32),
    CHECK(received_at > 0)
);

-- Fast lookup: "do I have any orphan that names HASH as its parent?"
-- Used during AcceptOrQueue to promote waiting children.
CREATE INDEX IF NOT EXISTS inbox_entries_by_parent
    ON inbox_entries(peer_id, prev_hash);

CREATE TABLE IF NOT EXISTS inbox_orphans (
    peer_id     BLOB    NOT NULL,
    hash        BLOB    NOT NULL,
    prev_hash   BLOB    NOT NULL,
    payload     BLOB    NOT NULL,
    received_at INTEGER NOT NULL,
    PRIMARY KEY (peer_id, hash),
    CHECK(length(hash) = 32),
    CHECK(length(prev_hash) = 32),
    CHECK(received_at > 0)
);

CREATE INDEX IF NOT EXISTS inbox_orphans_by_parent
    ON inbox_orphans(peer_id, prev_hash);

-- Used for the per-peer orphan eviction policy in future tickets and for
-- the global cap check in P1-5.
CREATE INDEX IF NOT EXISTS inbox_orphans_by_received_at
    ON inbox_orphans(received_at);

CREATE TABLE IF NOT EXISTS inbox_known_chain_head (
    peer_id    BLOB    PRIMARY KEY,
    last_hash  BLOB    NOT NULL,
    updated_at INTEGER NOT NULL,
    CHECK(length(last_hash) = 32),
    CHECK(updated_at > 0)
);
