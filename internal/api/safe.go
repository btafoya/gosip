package api

import "log/slog"

// safeGo spawns a goroutine that recovers from panics to prevent
// crashing the entire server on async task failures.
func safeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("goroutine panic recovered", "panic", r)
			}
		}()
		fn()
	}()
}
