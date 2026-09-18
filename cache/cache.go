package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type Result struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	Timestamp string `json:"timestamp"`
}

type cacheData struct {
	Results []Result `json:"results"`
}

type SprayCache struct {
	mu       sync.Mutex
	filePath string
	tried    map[string]bool
	data     cacheData
	dirty    bool
}

func cacheKey(username, password string) string {
	return username + "\x00" + password
}

func New(filePath string) (*SprayCache, error) {
	c := &SprayCache{
		filePath: filePath,
		tried:    make(map[string]bool),
	}

	if _, err := os.Stat(filePath); err == nil {
		if err := c.load(); err != nil {
			return nil, fmt.Errorf("failed to load cache: %w", err)
		}
	}

	return c, nil
}

func (c *SprayCache) load() error {
	raw, err := os.ReadFile(c.filePath)
	if err != nil {
		return err
	}

	var d cacheData
	if err := json.Unmarshal(raw, &d); err != nil {
		return fmt.Errorf("corrupt cache file: %w", err)
	}

	for _, r := range d.Results {
		c.tried[cacheKey(r.Username, r.Password)] = true
	}
	c.data = d
	return nil
}

func (c *SprayCache) AlreadyTried(username, password string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tried[cacheKey(username, password)]
}

func (c *SprayCache) Record(username, password string, success bool, errMsg string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.tried[cacheKey(username, password)] = true
	c.data.Results = append(c.data.Results, Result{
		Username:  username,
		Password:  password,
		Success:   success,
		Error:     errMsg,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
	c.dirty = true

	return c.flush()
}

func (c *SprayCache) flush() error {
	raw, err := json.Marshal(&c.data)
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}
	if err := os.WriteFile(c.filePath, raw, 0600); err != nil {
		return fmt.Errorf("failed to write cache: %w", err)
	}
	c.dirty = false
	return nil
}

func (c *SprayCache) TriedCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.tried)
}

func (c *SprayCache) SuccessCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	for _, r := range c.data.Results {
		if r.Success {
			count++
		}
	}
	return count
}

func (c *SprayCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.dirty {
		return c.flush()
	}
	return nil
}
