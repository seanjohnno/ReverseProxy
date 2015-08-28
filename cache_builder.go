package reverseproxy

import (
	"github.com/seanjohnno/memcache"
)

const (
	LRUCache 	= "lru"
	Empty		= ""
)

type CacheBuilder struct {
	CacheMap map[string]memcache.Cache
}

func CreateCacheBuilder() *CacheBuilder {
	return &CacheBuilder { CacheMap: make(map[string]memcache.Cache) }
}

func (this *CacheBuilder) CreateCache(cacheName string, cacheType string, cacheLimit int) memcache.Cache {
	if cacheLimit > 0 {
		
		// We have cacheName so we want to check if its already been created
		if cacheName != "" {
			
			// It its present we can return it
			if c, OK := this.CacheMap[cacheName]; OK {
				return c

			// If its not present then create and add to hash
			} else {
				c = this.CreateCacheAlgol(cacheType, cacheLimit)
				this.CacheMap[cacheName] = c
				return c
			}

		// No CacheName so we just create (don't need to add it to our map as it doesn't have a name so it can't be shared)
		} else {
			return this.CreateCacheAlgol(cacheType, cacheLimit)
		}
	}
	return nil
}

func (this *CacheBuilder) CreateCacheAlgol(cacheType string, limit int) memcache.Cache {
	switch cacheType {
		// TODO - Need to add a map to memcache
	case LRUCache:
		return memcache.CreateLRUCache(limit)
	case Empty:
		panic("You need to specify a cache strategy")
	default:
		panic("Unknown cache strategy")
	}
}