package reverseproxy

import (
	"github.com/seanjohnno/memcache"
	"errors"
)

const (
	// LRUCache constant to indicate we want an lru implementation
	LRUCache 	= "lru"

	// Empty string
	Empty		= ""
)

type CacheBuilder interface {
	CreateCache(cacheName string, cacheType string, cacheLimit int) (memcache.Cache, error)
}

// CacheBuilder is the struct we use to map and store Cache instances
//
// It's used so we can share cache objects across ServerBlocks (optional)
type CacheBuilderImpl struct {

	// CacheMap is used to map a cache name to a Cache instance
	CacheMap map[string]memcache.Cache
}

// CreateCacheBuilder returns a new CacheBuilder struct
func CreateCacheBuilder() CacheBuilder {
	return &CacheBuilderImpl { CacheMap: make(map[string]memcache.Cache) }
}

// CreateCache returns a Cache instance and stores it in our CacheBuilder object
//
// If a cache with the same name already exists it just returns it 
func (this *CacheBuilderImpl) CreateCache(cacheName string, cacheType string, cacheLimit int) (memcache.Cache, error) {
	if cacheLimit > 0 {
		
		// We have cacheName so we want to check if its already been created
		if cacheName != "" {
			
			// It its present we can return it
			if c, OK := this.CacheMap[cacheName]; OK {
				return c, nil

			// If its not present then create and add to hash
			} else {
				c, err := this.CreateCacheAlgol(cacheType, cacheLimit)
				if err == nil {
					this.CacheMap[cacheName] = c
				}
				return c, err
			}

		// No CacheName so we just create (don't need to add it to our map as it doesn't have a name so it can't be shared)
		} else {
			return this.CreateCacheAlgol(cacheType, cacheLimit)
		}
	}
	return nil, errors.New("Zero sized cache")
}

// CreateCacheAlgol creates the cache algorithm implementation
func (this *CacheBuilderImpl) CreateCacheAlgol(cacheType string, limit int) (memcache.Cache, error) {
	switch cacheType {
	case LRUCache:
		return memcache.CreateLRUCache(limit), nil
	case Empty:
		return nil, errors.New("You need to specify a cache strategy")
	default:
		return nil, errors.New("Unknown cache strategy")
	}
}