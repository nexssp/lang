package core

import "context"

type contextKey struct{}

var autoInstallDisabledKey contextKey

// WithAutoInstallDisabled marks a context so Resolve and execution report
// missing interpreters without invoking package managers.
func WithAutoInstallDisabled(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, autoInstallDisabledKey, true)
}

// AutoInstallDisabled reports whether automatic interpreter installation is
// disabled in ctx.
func AutoInstallDisabled(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	disabled, _ := ctx.Value(autoInstallDisabledKey).(bool)
	return disabled
}

func autoInstallDisabled(ctx context.Context) bool { return AutoInstallDisabled(ctx) }
