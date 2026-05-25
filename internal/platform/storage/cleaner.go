package storage

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

// StartCleanupWorker เป็น Background Job เช็คไฟล์ขยะตามเวลาที่กำหนด
func StartCleanupWorker(targetDir string, maxAge time.Duration, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			entries, err := os.ReadDir(targetDir)
			if err != nil {
				continue
			}

			now := time.Now()
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}

				info, err := entry.Info()
				if err != nil {
					continue
				}

				// ถ้าไฟล์เก่าเกิน maxAge ให้ลบทิ้ง
				if now.Sub(info.ModTime()) > maxAge {
					path := filepath.Join(targetDir, entry.Name())
					if err := os.Remove(path); err == nil {
						log.Printf("🗑️ [Storage] Deleted old file: %s\n", entry.Name())
					}
				}
			}
		}
	}()
}
