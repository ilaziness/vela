package update

import (
	"sync/atomic"

	"github.com/ilaziness/vela/internal/updater"
)

// Deps are the services the update dialog needs.
type Deps struct {
	Manager     *updater.Manager
	OnInstalled func()
	// Prompted is set once an update dialog has been shown, so the
	// delayed background check does not pop a second dialog after a
	// manual About-page check.
	Prompted atomic.Bool
}
