package nexssflow

import (
	"context"

	core "github.com/nexssp/lang/core"
)

// WithAutoInstallDisabled is the adapter-level convenience wrapper used by
// compile/test hosts that must never invoke a package manager.
func WithAutoInstallDisabled(ctx context.Context) context.Context {
	return core.WithAutoInstallDisabled(ctx)
}
