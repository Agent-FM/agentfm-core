-- Schema v4 — symmetry fix for equivocators table.
--
-- Migration 003 added an UPDATE trigger but forgot DELETE. Without
-- this, a local operator with sqlite3 could DELETE FROM equivocators
-- WHERE peer_id = X to unmark someone locally. The mesh-wide marker
-- still exists on other peers, so the offender remains ejected
-- everywhere else — but the local boss would stop ejecting them.
--
-- This migration closes that local inconsistency. The marker is now
-- permanent at the database layer too, matching the entries-table
-- pattern.

CREATE TRIGGER IF NOT EXISTS equivocators_no_delete
BEFORE DELETE ON equivocators
BEGIN
    SELECT RAISE(ABORT, 'equivocators is append-only: DELETE refused');
END;
