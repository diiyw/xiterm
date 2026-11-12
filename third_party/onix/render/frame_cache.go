package render

const (
	frameCacheIdleFrames = 120
	frameCacheTrimBudget = 64
)

// frameCache retains the active working set without scanning it every frame.
// Values own CPU metadata only; release must unlink their external references.
// GPU resources referenced by submitted work are never destroyed here.
type frameCache[K comparable, V any] struct {
	entries        map[K]*frameCacheEntry[K, V]
	oldest, newest *frameCacheEntry[K, V]
	capacity       int // Entry high-water mark/reservation hint, not Go bucket bytes.
	created        int
	retention      bufferRetention
}

type frameCacheEntry[K comparable, V any] struct {
	key            K
	value          V
	previous, next *frameCacheEntry[K, V]
	frame          uint64
}

func (c *frameCache[K, V]) getOrCreate(key K, frame uint64) (*frameCacheEntry[K, V], bool) {
	if entry := c.entries[key]; entry != nil {
		if entry.frame != frame {
			entry.frame = frame
			if c.newest != entry {
				c.unlink(entry)
				c.linkNewest(entry)
			}
		}
		return entry, false
	}
	if c.entries == nil {
		c.entries = make(map[K]*frameCacheEntry[K, V])
	}
	entry := &frameCacheEntry[K, V]{key: key, frame: frame}
	c.entries[key] = entry
	c.created++
	c.capacity = max(c.capacity, len(c.entries))
	c.linkNewest(entry)
	return entry, true
}

func (c *frameCache[K, V]) linkNewest(entry *frameCacheEntry[K, V]) {
	entry.previous, entry.next = c.newest, nil
	if c.newest != nil {
		c.newest.next = entry
	} else {
		c.oldest = entry
	}
	c.newest = entry
}

func (c *frameCache[K, V]) unlink(entry *frameCacheEntry[K, V]) {
	if entry.previous != nil {
		entry.previous.next = entry.next
	} else {
		c.oldest = entry.next
	}
	if entry.next != nil {
		entry.next.previous = entry.previous
	} else {
		c.newest = entry.previous
	}
}

func (c *frameCache[K, V]) remove(entry *frameCacheEntry[K, V], release func(K, *V)) {
	c.unlink(entry)
	delete(c.entries, entry.key)
	if release != nil {
		release(entry.key, &entry.value)
	}
	// Clear even when another owner still temporarily holds the entry pointer.
	*entry = frameCacheEntry[K, V]{}
}

func (c *frameCache[K, V]) trim(frame uint64, release func(K, *V)) {
	// Match sustained churn as well as draining old bursts. A fixed budget
	// alone would leak stale entries when new combinations arrive faster.
	budget := max(frameCacheTrimBudget, c.created)
	c.created = 0
	for count := 0; count < budget; count++ {
		entry := c.oldest
		if entry == nil || frame-entry.frame <= frameCacheIdleFrames {
			break
		}
		c.remove(entry, release)
	}
	// Finish a bounded eviction backlog before starting the low-load window;
	// otherwise its declining counts cause repeated intermediate map rebuilds.
	if entry := c.oldest; entry != nil && frame-entry.frame > frameCacheIdleFrames {
		c.retention = bufferRetention{}
		return
	}
	// Go maps retain deleted buckets. Rebuild only after sustained underuse,
	// walking the remaining entries once and retaining their stable addresses.
	if capacity := c.retention.trimCapacity(c.capacity, len(c.entries), 32); capacity != 0 {
		if len(c.entries) == 0 {
			c.entries = nil
			c.capacity = 0
		} else {
			entries := make(map[K]*frameCacheEntry[K, V], capacity)
			for entry := c.oldest; entry != nil; entry = entry.next {
				entries[entry.key] = entry
			}
			c.entries, c.capacity = entries, capacity
		}
	}
}

func (c *frameCache[K, V]) clear(release func(K, *V)) {
	for c.oldest != nil {
		c.remove(c.oldest, release)
	}
	*c = frameCache[K, V]{}
}
