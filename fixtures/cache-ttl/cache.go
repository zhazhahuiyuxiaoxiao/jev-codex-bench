package cachettl

import "time"

type entry struct {
	value   string
	expires time.Time
}

type Cache struct {
	now   func() time.Time
	items map[string]entry
}

func New(now func() time.Time) *Cache {
	return &Cache{now: now, items: make(map[string]entry)}
}

func (c *Cache) Set(key, value string, ttl time.Duration) {
	c.items[key] = entry{value: value, expires: c.now().Add(ttl)}
}

func (c *Cache) Get(key string) (string, bool) {
	item, ok := c.items[key]
	if !ok {
		return "", false
	}
	if item.expires.Before(c.now()) {
		delete(c.items, key)
		return "", false
	}
	return item.value, true
}
