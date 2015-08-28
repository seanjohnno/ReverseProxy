package reverseproxy

import (
	"os"
	"github.com/seanjohnno/memcache"
)

const (
	CompressionSuffix = "gzip"
)

type CacheFileLoader struct {

	// FileRetriever is the next in the chain to pass request onto if we can't find in cache 
	WrappedRetriever FileRetriever

	// UnderlyingCache is the cache impl we're using to store/retrieve the file content
	UnderlyingCache memcache.Cache
}

func (this *CacheFileLoader) GetFile(filePath string, resource *ServerResource, compression bool) (*FileContent, error) {
	if fc := this.GetFileInCache(filePath, compression); fc == nil {
		if fc, err := this.WrappedRetriever.GetFile(filePath, resource, compression); err == nil {

			if fc.Compression {
				filePath = filePath + CompressionSuffix
			}
			this.UnderlyingCache.Add(filePath, fc)
			
			return fc, nil
		} else {
			return fc, err
		}
	} else {
		return fc, nil
	}
}

// GetFile retrieves cached file (FileCacheItem) if its been added and isn't stale (by comparing stored timestamp)
func (this *CacheFileLoader) GetFileInCache(filePath string, compression bool) (*FileContent) {

	// Check is cache is already present
	if fileCacheItem, present := this.CheckFileInCache(filePath, compression); present {
		
		// Grab the files FileInfo
		if curFileInfo, err := os.Stat(fileCacheItem.AbsolutePath); err == nil {

			// If file modTime is the same then we can return data
			if fileCacheItem.FileInfo.ModTime().Equal( curFileInfo.ModTime() ) {
				Debug("File found in cache: " + fileCacheItem.AbsolutePath)
				return fileCacheItem
			
			// File modTime has changed so file has changed, remove from cache
			} else {
				this.UnderlyingCache.Remove(filePath)
			}

		// Problem getting fileInfo...
		} else {
			this.UnderlyingCache.Remove(filePath)
		}
	}
	Debug("File not found in cache: " + filePath)
	return nil
}

func (this *CacheFileLoader) CheckFileInCache(filePath string, compression bool) (*FileContent, bool) {

	// Check if we're looking for compressed content
	if compression {

		// Use compression suffix (to discern from non-compressed content)
		if content, ok := this.UnderlyingCache.Get(filePath + CompressionSuffix); ok {
			return content.(*FileContent), ok
		
		// Compressed doesn't exist so lets check for normal...
		} else if content, ok := this.UnderlyingCache.Get(filePath); ok {
			
			// ...and make sure the IgnoreCompression flag is set (for non-text content)
			ret := content.(*FileContent)
			if ret.IgnoreCompression {
				return ret, true
			} else {
				return nil, false
			}

		} else {
			return nil, false
		}

	// Check for non-compressed in cache
	} else  if content, ok := this.UnderlyingCache.Get(filePath); ok {
		ret := content.(*FileContent)
		return ret, ok
	}

	return nil, false
}