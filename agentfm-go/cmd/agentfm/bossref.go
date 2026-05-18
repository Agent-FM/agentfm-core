package main

import (
	"sync/atomic"

	"agentfm/internal/boss"
)

// currentBossRef holds the live Boss pointer so HTTP handler
// closures built BEFORE boss.NewWithOptions returns can call into
// it. The bootstrap helper builds opts (including the comment
// submission handler closure) and only THEN constructs the boss
// — the handler stores the boss reference here via the AttachBoss
// helper below.
//
// Using atomic.Pointer keeps the read path in the hot HTTP request
// handler lock-free.
var currentBossRef atomic.Pointer[boss.Boss]

// AttachBoss stores b for handler closures that need a late-bound
// boss reference. Call this immediately after boss.NewWithOptions
// returns, before StartAPIServer.
func AttachBoss(b *boss.Boss) {
	currentBossRef.Store(b)
}
