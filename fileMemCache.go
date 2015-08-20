package reverseproxy

import (
	"time"
	"os"
	"github.com/seanjohnno/memcache"
)

const (
	CompressionSuffix			= "compressed"
)

// ------------------------------------------------------------------------------------------------------------------------
// Struct: FileCacheItem
// Implements: CacheItem (memcache.go)
// ------------------------------------------------------------------------------------------------------------------------

// FileCacheItem is used to store the contents of a file and its modified date in memory for faster access
type FileCacheItem struct {

	// Data is the content of the file (potentially compressed)
	Data []byte

	// FileModTime is a timestamp from the FileInfo object
	//
	// Used to see if the cache is stale
	FileModTime time.Time
}

// Size is used to tell the cache how big this item is in bytes
//
// Implementing the CacheItem interface
func (this *FileCacheItem) Size() int {
	return len(this.Data)
}

// ------------------------------------------------------------------------------------------------------------------------
// Struct: FileCacheAcessor
// ------------------------------------------------------------------------------------------------------------------------

// FileCacheAcessor wraps a caching algol to store and retrieve in-memory file content
type FileCacheAcessor struct {

	// FilePath represents the path/name of the file
	//
	// Used as part of the cache key so needs to be unique, can't just be filename
	FilePath string

	// FileInfo holds information about the file on the fs
	FileInfo os.FileInfo

	// Compression indicates whether the content is compressed or not
	Compression bool

	// UnderlyingCache is the cache algol we're using to store/retrieve the file content
	UnderlyingCache memcache.Cache

	// CacheKey is used to get/set the data in the cache
	CacheKey string
}

// NewFileCache create
func NewFileCache(filePath string, fileInfo os.FileInfo, compression bool, cache memcache.Cache) (*FileCacheAcessor) {
	if cache == nil {
		return &FileCacheAcessor{}
	} else {
		fca := &FileCacheAcessor { FilePath: filePath, FileInfo: fileInfo, Compression: compression, UnderlyingCache: cache }
		
		// Create the key (we need separate values for compressed / not compressed)
		if(compression) {
			fca.CacheKey = (filePath + CompressionSuffix)
		} else {
			fca.CacheKey = filePath
		}

		return fca
	}
}

// GetFile retrieves cached file (FileCacheItem) if its been added and isn't stale (by comparing stored timestamp)
func (this *FileCacheAcessor) GetFile() []byte {
	if this.UnderlyingCache != nil {
		// Check if the item is present
		if cacheItem, OK := this.UnderlyingCache.Get(this.CacheKey); OK {
			item := cacheItem.(*FileCacheItem)

			// If cache is stale (by comparing file timestamps) then we remove it
			if !this.FileInfo.ModTime().Equal(item.FileModTime) {
				this.UnderlyingCache.Remove(this.CacheKey)
				Debug("+GetFile - Removing from cache: " + this.CacheKey)
				return nil
			}
			return item.Data
		}
	}
	return nil
}

// PutFile adds file content to the cache along with the current file timestamp
func (this *FileCacheAcessor) PutFile(data []byte, modTime time.Time) {
	if this.UnderlyingCache != nil {
		this.UnderlyingCache.Add(this.CacheKey, &FileCacheItem { Data: data, FileModTime: modTime} )
	}
}