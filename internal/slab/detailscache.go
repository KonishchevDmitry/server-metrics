package slab

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"sync"
)

type slabDetailsCache struct {
	lock  sync.Mutex
	cache map[string]bool
}

func newSlabDetailsCache() *slabDetailsCache {
	return &slabDetailsCache{
		cache: make(map[string]bool),
	}
}

func (c *slabDetailsCache) isReclaimable(name string) (bool, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if reclaimable, ok := c.cache[name]; ok {
		return reclaimable, nil
	}

	path := path.Join("/sys/kernel/slab", name, "reclaim_account")

	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	data = bytes.TrimRight(data, "\n")
	if len(data) != 1 || data[0] != '0' && data[0] != '1' {
		return false, fmt.Errorf("%s has an unexpected contents: %q", path, data)
	}

	reclaimable := data[0] == '1'
	c.cache[name] = reclaimable

	return reclaimable, nil
}
