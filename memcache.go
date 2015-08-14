package reverseproxy

import (
	"errors"
	"sync"
)

const (
	ErrorExceedsMaxSize = "Exceeds max size, can't store"
)

var (
	mutex = sync.Mutex { }
)

type Cache interface {
	Add(key string, val CacheItem) error
	Get(key string) (CacheItem, bool)
	Remove(key string)
}

type CacheItem interface {
	Size() int
}

type lruCache struct {
	keyValMap map[string]*lruCacheItem
	
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

// ----------------------------------------------------------------------------------------------------

func CreateLRUCache(maxsize int) (Cache) {
	return &lruCache { keyValMap: make(map[string]*lruCacheItem), maxSize: maxsize }
}

func (this *lruCache) Add(k string, v CacheItem) error {

	this.Remove(k)

	// Create item
	lruItem := &lruCacheItem { cacheItem: v, key: k }

	// Can't store if it already exceeds max size
	if v.Size() > this.maxSize {
		return errors.New(ErrorExceedsMaxSize)
	}

	// Lock method
	mutex.Lock()
	defer mutex.Unlock()

	for this.curSize + v.Size() > this.maxSize {
		delete(this.keyValMap, this.tail.key)
		this.curSize -= this.tail.cacheItem.Size()

		newTail := this.tail.prev
		if newTail != nil {
			newTail.next = nil
			this.tail = newTail
		} else {
			this.head = nil
			this.tail = nil
		}
	}

	// Add to map (locking as maps aren't thread safe)
	this.keyValMap[k] = lruItem

	if this.head == nil {
		this.head = lruItem
		this.tail = lruItem
	} else {
		lruItem.next = this.head
		this.head.prev = lruItem
		this.head = lruItem
	}

	return nil
}

func (this *lruCache) Get(key string) (CacheItem, bool) {
	
	// Lock method
	mutex.Lock()
	defer mutex.Unlock()

	val, containsKey := this.keyValMap[key]

	if containsKey {
		if this.head != val {
			val.prev.next = val.next

			if val.next != nil {
				val.next.prev = val.prev
			}

			val.prev = nil
			val.next = this.head
			this.head.prev = val
			this.head = val
		}
		return val.cacheItem, containsKey
	}
	return nil, false
}

func (this *lruCache) Remove(key string) {
	// Lock method
	mutex.Lock()
	defer mutex.Unlock()

	// Check if cache is still here
	lruCacheItem, present := this.keyValMap[key]
	if present {
		delete(this.keyValMap, key)

		this.curSize -= lruCacheItem.cacheItem.Size()

		// If the node is the head of the linked list
		if lruCacheItem == this.head {
			if this.head.next != nil {
				lruCacheItem.next.prev = nil
				this.head = lruCacheItem.next
			} else {
				this.head = nil
				this.tail = nil
			}

		// If the node is the tail of the linked list
		} else if lruCacheItem == this.tail {
			
			if this.tail.prev != nil {
				lruCacheItem.prev.next = nil
				this.tail = lruCacheItem.prev
			} else {
				this.head = nil
				this.tail = nil
			}

		// If the nodes in the middle then join the neighbours up
		} else {
			lruCacheItem.prev.next = lruCacheItem.next
			lruCacheItem.next.prev = lruCacheItem.prev
		}
	}
}