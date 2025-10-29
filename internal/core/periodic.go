package core

import (
	"context"
	"time"
)

func Periodic(ctx context.Context, duration time.Duration, f func() error) {
	ticker := time.NewTicker(duration)
	defer ticker.Stop()

	// Run immediately on first call
	err := f()
	if err != nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		err = f()
		if err != nil {
			return
		}
	}
}
