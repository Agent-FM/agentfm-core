-- Schema v3 — equivocators table (P2-3).
--
-- Append-only roster of peers caught equivocating (showing two
-- non-extending heads). The marker is permanent in the schema; only
-- manual operator action via `agentfm reputation rehab` (P3-7) can
-- remove it, and that action itself is recorded in the ledger as a
-- new entry — there is no silent "unmark".

CREATE TABLE IF NOT EXISTS equivocators (
    peer_id    BLOB    PRIMARY KEY,
    alert_blob BLOB    NOT NULL,   -- serialised pb.EquivocationAlert
    marked_at  INTEGER NOT NULL,   -- unix ns when this node accepted the alert
    CHECK(marked_at > 0)
);

CREATE INDEX IF NOT EXISTS equivocators_by_marked_at
    ON equivocators(marked_at);

-- Refuse UPDATE: an existing equivocator marker is permanent. Future
-- additional alerts about the same peer are no-op INSERT-ON-CONFLICT.
CREATE TRIGGER IF NOT EXISTS equivocators_no_update
BEFORE UPDATE ON equivocators
BEGIN
    SELECT RAISE(ABORT, 'equivocators is append-only: UPDATE refused');
END;
