package reverseproxy

import (
	"os"
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
	"fmt"
)

//  Request / Response headers + value indicating whether client can accept compressed content and whether returned content is compressed
const (
	HeaderAcceptEncoding 	= "Accept-Encoding"
	HeaderContentEncoding 	= "Content-Encoding"
	CompressionGzip			= "gzip"
)

// Request / Response headers for caching content
const (
	HeaderIfModifiedSince 	= "If-Modified-Since"

	HeaderLastModified 		= "Last-Modified"
	HeaderExpires 			= "Expires"

	HeaderCacheControl 		= "Cache-Control"
	ValueCacheControl		= "must-revalidate, private"
	ValueExpires 			= "-1"
)

// Response header + values for type of content returned from server
// HTML extension. Files requested without extension are assume to be .html 
const(
	HeaderContentType 		= "Content-Type"
	HtmlExtension			= ".html"
	PlainTextMimeType		= "text/plain"
)

var (
	mimeMap = map[string]string {
		".html": "text/html",
		",css": "text/css",	
		".js": "text/javascript",
		".ico": "image/x-icon",
		".jpg": "image/jpeg",
		".jpeg": "image/jpeg",
		".png": "image/png",
		".gif": "image/gif",
	}
)

const(
	HttpNotModified			= 304
)

var (
	GMTLoc, GMTErr = time.LoadLocation("GMT")
)

type FileCacheItem struct {
	Data []byte
	FileModTime time.Time
}

func (this *FileCacheItem) Size() int {
	return len(this.Data)
}

// FileHandler combines "path" in server config block with the request path and attempts to fetch the file.
// It'll attempt to use gzip compression if the client accepts it. Stores the file in memory if the cache
// limit allows and also uses caching headers + If-Modified-Since so content doesn't need to be served
// again
func (sh *ServerHandler) FileHandler(w http.ResponseWriter, req *http.Request, res *ServerResource) {

	url := createUrl(req, res)
	setContentTypeHeader(w, url)

	// Check If-Modified-Since
	fileInfo, fileErr := os.Stat(*url)
	if fileErr == nil && !isModifiedSince(req, url, fileInfo) {
		w.WriteHeader(HttpNotModified)
		return
	} else {
		w.Header()[HeaderExpires] = []string{ ValueExpires }
		w.Header()[HeaderCacheControl] = []string{ ValueCacheControl }
		w.Header()[HeaderLastModified] = []string{ fileInfo.ModTime().In(GMTLoc).Format(time.RFC1123) }
	}

	// Check if we should be using compression or not + set header
	compressionTypes, acceptsCompression := req.Header[HeaderAcceptEncoding]
	useCompression := res.Compression && acceptsCompression && containsInArray(compressionTypes, CompressionGzip)
	if useCompression {
		w.Header()[HeaderContentEncoding] = []string{CompressionGzip}
	}

	// Create cache key
	fileCacheItem, cacheKey := retrieveCache(res, url)

	// Check if cache is stale
	if(fileCacheItem != nil && !fileInfo.ModTime().Equal(fileCacheItem.FileModTime)) {
		fileCacheItem = nil
		res.Cache.Remove(cacheKey)
	}

	var fileContent []byte

	// If we don't have cache or cache is stale then reload from filesystem
	if fileCacheItem == nil {
		// Buffer to hold file content
		f, err := os.Open(*url)
		defer f.Close()
		if err != nil {
			panic("Implement 404 handler")
		}
		
		// Read all of file content
		fileContent, err = ioutil.ReadAll(f)
		if err != nil {
			panic(err)
		}

		// Use compression
		if useCompression {
			buf := bytes.NewBuffer( make([]byte, 0) )
			compressionWriter := gzip.NewWriter(buf)
			_, err := compressionWriter.Write(fileContent)
			if err != nil {
				panic(err)
			}
			compressionWriter.Close()
			fileContent = buf.Bytes()
		}

		if cacheKey != "" {
			fmt.Println("Adding to cache")
			res.Cache.Add( cacheKey, &FileCacheItem{ Data: fileContent, FileModTime: fileInfo.ModTime() } )
		}
	} else {
		fmt.Println("Fetching from cache")
		fileContent = fileCacheItem.Data
	}

	// TODO - Need to check returned int against size?
	_, writeErr := w.Write(fileContent)
	if writeErr != nil {
		panic("Implement 500 handler - Writer error")
	}
}

func retrieveCache(res *ServerResource, url *string) (cacheItem *FileCacheItem, cacheKey string) {

	if res.Cache != nil {

		// Create cache key
		var urlBuffer bytes.Buffer
		urlBuffer.WriteString(*url)
		if res.Compression {
			urlBuffer.WriteString(CompressionGzip)
		}

		cacheKey = urlBuffer.String()
		ci, present := res.Cache.Get(cacheKey)
		if present {
			cacheItem = ci.(*FileCacheItem)
		}
		return
	} else {
		cacheItem = nil
	}
	return
}

func createUrl(req *http.Request, res *ServerResource) *string {
	// Concatenate path from config with path supplied in URL
	var urlBuffer bytes.Buffer
	urlBuffer.WriteString(res.Path)
	urlBuffer.WriteString(req.URL.Path)

	// No extension so lets assume .html
	if !strings.Contains(req.URL.Path, ".") {
		urlBuffer.WriteString(HtmlExtension)
	}
	url := urlBuffer.String()

	fmt.Println(url)

	return &url
}

func isModifiedSince(req *http.Request, url *string, fi os.FileInfo) bool {
	modifiedSince, msPresent := req.Header[HeaderIfModifiedSince]
	if msPresent && len(modifiedSince) > 0 {
		ms := modifiedSince[0]

		var parsedTime time.Time
		var err error
		// http://www.w3.org/Protocols/rfc2616/rfc2616-sec3.html (3.3 Date/Time Formats)
		switch ms[3] {
			// RFC 822, updated by RFC 1123 - Sun, 06 Nov 1994 08:49:37 GMT
			case ',':
				parsedTime, err = time.Parse(time.RFC1123, ms)
			// ANSI C's asctime() format - Sunday, 06-Nov-94 08:49:37 GMT
			case ' ':
				parsedTime, err = time.Parse(time.ANSIC, ms)
			// RFC 850, obsoleted by RFC 1036 - Sun Nov  6 08:49:37 1994
			default:
				parsedTime, err = time.Parse(time.RFC850, ms)
		}

		// Can only continue with this if we have a valid date
		if err == nil {
			if fi.ModTime().Truncate(time.Second).Equal(parsedTime) {
				return false
			}
		}
	}
	return true
}

func containsInArray(vals []string, str string) bool {
	for _, val := range vals {
		if strings.Index(val, str) != -1 {
			return true
		}
	}
	return false
}

func setContentTypeHeader(w http.ResponseWriter, url *string) {

	for key, val := range mimeMap {
		if strings.HasSuffix(*url, key) {
			w.Header()[HeaderContentType] = []string{val}
			return
		}
	}

	w.Header()[HeaderContentType] = []string{PlainTextMimeType}
}