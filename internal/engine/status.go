package engine

import "github.com/danieljustus/symaira-vibecoder/internal/config"

// allowed is the guarded status transition table. The engine's own flow only
// ever uses legal transitions; the table doubles as a safety net for
// GUI-initiated status writes (re-run a done step, re-enable a skipped one).
var allowed = map[config.StepStatus]map[config.StepStatus]bool{
	config.StatusPending: {
		config.StatusInProgress: true,
		config.StatusSkipped:    true,
		config.StatusBlocked:    true,
	},
	config.StatusInProgress: {
		config.StatusDone:        true,
		config.StatusFailed:      true,
		config.StatusNeedsReview: true,
		config.StatusBlocked:     true,
		config.StatusPending:     true, // crash/cancel reset
	},
	config.StatusBlocked: {
		config.StatusPending: true,
		config.StatusSkipped: true,
	},
	config.StatusFailed: {
		config.StatusPending:    true, // retry
		config.StatusInProgress: true,
		config.StatusSkipped:    true,
	},
	config.StatusNeedsReview: {
		config.StatusDone:       true, // human ack
		config.StatusPending:    true,
		config.StatusInProgress: true,
		config.StatusSkipped:    true,
	},
	config.StatusSkipped: {
		config.StatusPending: true, // re-enable
	},
	config.StatusDone: {
		config.StatusPending:    true, // re-run
		config.StatusInProgress: true,
	},
}

// canTransition reports whether moving from -> to is legal. Same-state and the
// empty (pending) normalization are always allowed.
func canTransition(from, to config.StepStatus) bool {
	from = from.Effective()
	if from == to {
		return true
	}
	m, ok := allowed[from]
	return ok && m[to]
}
