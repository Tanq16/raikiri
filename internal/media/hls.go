package media

import (
	"context"
	"os"
	"time"
)

func WaitForFile(ctx context.Context, path string, attempts int, sleep time.Duration) bool {
	for range attempts {
		info, err := os.Stat(path)
		if err == nil && info.Size() > 0 {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(sleep):
		}
	}
	return false
}
