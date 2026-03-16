package media

import (
	"os"
	"time"
)

func WaitForFile(path string, attempts int, sleep time.Duration) bool {
	for i := 0; i < attempts; i++ {
		info, err := os.Stat(path)
		if err == nil && info.Size() > 0 {
			return true
		}
		time.Sleep(sleep)
	}
	return false
}
