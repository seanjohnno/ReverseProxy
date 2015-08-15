package reverseproxy

import (
	"os"
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
	"strconv"
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

var (
	HttpNotModified			= 304
	HttpNotFound 			= 404
	HttpInternalServerError = 500
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
	// Combine fs path + request path to create absolute path
	path := createUrl(&(req.URL.Path), res)
	fileHandler(w, req, res, path, true)
}

func fileHandler(w http.ResponseWriter, req *http.Request, res *ServerResource, path *string, tryErrorPages bool) {
	// Set content-type based on extension
	setContentTypeHeader(w, path)
	
	// Attempt to open file info
	fileInfo, fileErr := os.Stat(*path);
	if fileErr != nil {
		handleError(w, req, res, HttpNotFound, tryErrorPages)
		return

	// If client already has file then return not modified, no need to write body
	} else if !isModifiedSince(req, path, fileInfo) {
		w.WriteHeader(HttpNotModified)
		return

	// Set cache headers so clients with subsequently send If-Modified-Since header
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

	// Get cache key, check if stale by comparing store modified time, remove if so
	fileCacheItem, cacheKey := retrieveCache(res, path)
	if fileCacheItem != nil && !fileInfo.ModTime().Equal(fileCacheItem.FileModTime) {
		fileCacheItem = nil
		res.Cache.Remove(cacheKey)
	}

	var fileContent []byte

	// If we have cache then we can just set data here
	if fileCacheItem != nil {
		fileContent = fileCacheItem.Data

	// If we don't have cache (or cache was stale) then reload from filesystem	
	} else {
		if fileContent, fileErr = loadResourceFromFS(path, useCompression); fileErr != nil {
			handleError(w, req, res, HttpInternalServerError, tryErrorPages)
			return
		}

		if cacheKey != "" {
			res.Cache.Add( cacheKey, &FileCacheItem{ Data: fileContent, FileModTime: fileInfo.ModTime() } )
		}
	}

	// Write response body
	if _, writeErr := w.Write(fileContent); writeErr != nil {
		handleError(w, req, res, HttpInternalServerError, tryErrorPages)
		return
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

func createUrl(requestPath *string, res *ServerResource) *string {
	// Concatenate path from config with path supplied in URL
	var urlBuffer bytes.Buffer
	urlBuffer.WriteString(res.Path)
	urlBuffer.WriteString(*requestPath)

	// If we finish in a slash then we're a directory and we need a default file
	if strings.HasSuffix(*requestPath, "/") {
		urlBuffer.WriteString(res.DefaultFile)

	// No extension so lets assume .html
	} else if !strings.Contains(*requestPath, ".") {
		urlBuffer.WriteString(res.DefaultExtension)
	}
	url := urlBuffer.String()

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

func handleError(w http.ResponseWriter, req *http.Request, res *ServerResource, error int, allowErrorRedirect bool) {

	if allowErrorRedirect {

		// See if we have a specific file for the error
		errStr := strconv.Itoa(error)
		path, hasErrorPage := res.Error[errStr]
		
		// No specific error handler so check for generic
		if !hasErrorPage {
			var pathBuffer bytes.Buffer
			pathBuffer.WriteString(errStr[:len(errStr)-1])
			pathBuffer.WriteString("x")
			path, hasErrorPage = res.Error[pathBuffer.String()]
		}

		if hasErrorPage {
			errorPath := createUrl(&path, res)
			fileHandler(w, req, res, errorPath, false)
			return
		}

	}
	w.WriteHeader(error)
}

func loadResourceFromFS(path *string, useCompression bool) ([]byte, error) {
	// Buffer to hold file content
	f, err := os.Open(*path)
	defer f.Close()
	if err != nil {
		return nil, err
	}
	
	// Read all of file content
	fileContent, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	// Use compression
	if useCompression {
		buf := bytes.NewBuffer( make([]byte, 0) )
		
		compressionWriter := gzip.NewWriter(buf)
		_, err := compressionWriter.Write(fileContent)
		compressionWriter.Close()
		if err != nil {
			return nil, err
		}
		
		fileContent = buf.Bytes()
	}
	return fileContent, nil
}