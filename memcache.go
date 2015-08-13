package reverseproxy

import (
	"errors"
)

const (
	ErrorExceedsMaxSize = "Exceeds max size, can't store"
)

type Cache interface {
	Add(key string, val CacheItem) error
	Get(key string) (CacheItem, bool)
}

type CacheItem interface {
	Size() int
}

type lruCache struct {
	keyValMap map[string]lruCacheItem
	head *lruCacheItem
	tail *lruCacheItem
	maxSize int
	curSize int
}

type lruCacheItem struct {
	cacheItem CacheItem
	key string
	prev *lruCacheItem
	next *lruCacheItem
}

func (this *lruCacheItem) removeAndJoinNeighbours() {
	if this.prev != nil {
		this.prev.next = this.next
	}
	if this.next != nil {
		this.next.prev = this.prev
	}
}

func CreateLRUCache(maxsize int) (Cache) {
	return &lruCache { keyValMap: make(map[string]lruCacheItem), maxSize: maxsize }
}

func (this *lruCache) Add(k string, val CacheItem) error {

	// Can't store if it already exceeds max size
	if val.Size() > this.maxSize {
		return errors.New(ErrorExceedsMaxSize)
	}

	// Remove older entries if we're going to exceed max size
	for val.Size() + this.curSize > this.maxSize {
		this.maxSize -= this.tail.cacheItem.Size()
		oldTail := this.tail
		this.tail = this.tail.prev
		
		if this.tail == nil {
			this.head = nil
		}

		delete(this.keyValMap, oldTail.key)
	}

	// 
	lruItem := lruCacheItem { cacheItem: val, key: k }
	this.insert(&lruItem)
	
	// Add to map
	this.keyValMap[k] = lruItem

	// Increase size
	this.curSize = this.curSize + val.Size()

	return nil
}

func (this *lruCache) Get(key string) (CacheItem, bool) {
	val, containsKey := this.keyValMap[key]
	if containsKey {
		val.removeAndJoinNeighbours()
		this.insert(&val)
		return val.cacheItem, true
	}
	return nil, false
}

func (this *lruCache) insert(val *lruCacheItem) {
	if this.head != nil {
		this.head.prev = val
		val.next = this.head
		this.head = val
	} else {
		this.head = val
		this.tail = val
	}
}