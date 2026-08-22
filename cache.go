package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// cacheFile is the on-disk shape. follower's fields are already exported, so
// it needs no struct tags of its own.
type cacheFile struct {
	FetchedAt time.Time  `json:"fetched_at"`
	Followers []follower `json:"followers"`
}

func cachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gh-followers", "followers.json"), nil
}

// readCache reports ok == false for every failure mode — missing, unreadable,
// corrupt, or empty. A bad cache is a cache miss, never an error the user sees.
// ponytail: writes are not atomic, so a process killed mid-write leaves a
// truncated file; that lands here as a miss and the next fetch overwrites it.
func readCache(path string) (cacheFile, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return cacheFile{}, false
	}
	var c cacheFile
	if err := json.Unmarshal(b, &c); err != nil {
		return cacheFile{}, false
	}
	if len(c.Followers) == 0 {
		return cacheFile{}, false
	}
	return c, true
}

func writeCache(path string, c cacheFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
