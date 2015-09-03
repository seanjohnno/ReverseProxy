package reverseproxy

import (
	"os"
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
)

var (
	// GMTLoc is a GTM Time struct, server needs to return GMT dates
	GMTLoc, _ = time.LoadLocation("GMT")
)

// ------------------------------------------------------------------------------------------------------------------------
// Exported functions
// ------------------------------------------------------------------------------------------------------------------------

type FSHandler struct {

	// BaseHandler contains ServerResource & ErrorMappings map
	BaseHandler

	// Cache is used to cache files in memory
	//
	// Non-nil if specified in ServerResource(config) and uses the underlying cache algorithm specified
	FileAccessor FileRetriever
}

// NewFSHandler returns an FSHandler
//
// It's initialised with a cache if specified in the ServerResource
func NewFSHandler(rsc *ServerResource, errorMappings []ErrorMapping, cacheBuilder CacheBuilder) (*FSHandler) {
	
	Debug(errorMappings)

	var fa FileRetriever
	fa = &FileSystemLoader{}
	
	// If a cache is specified then we can wrap our FileRetriever with a cache FileRetriever
	if rsc.Cache.Strategy != "" {
		if cache, err := cacheBuilder.CreateCache(rsc.Cache.Name, rsc.Cache.Strategy, rsc.Cache.Limit); cache != nil && err == nil {
			 fa = &CacheFileLoader{ WrappedRetriever: fa, UnderlyingCache: cache }
		}
	}

	return &FSHandler{ BaseHandler { rsc, errorMappings }, fa }
}

// ------------------------------------------------------------------------------------------------------------------------
// Exported functions
// ------------------------------------------------------------------------------------------------------------------------

// HandleRequest write files to response body
//
// It works by attempting to combine ServerResource.Path (from config) with the request path
// + defaulting extensions or files if they're missing (also from config)
func (this *FSHandler) HandleRequest(w http.ResponseWriter, req *http.Request) {

	Debug("+HandlerFS - Path: " + req.URL.Path)

	// Combine fs path + request path to create absolute path
	// Check if we should be using compression or not + set header
	useCompression := this.shouldUseCompression(req)
	if fc, err := this.FileAccessor.GetFile(req, this.Resource, useCompression); err == nil {
		this.writeFile(w, req, fc)
	} else {
		this.handleError(w, req, int(http.StatusNotFound), useCompression)
	}
}

// handleError will attempt to serve an error page instead of a status code
//
// If it has a handler for 
func (this *FSHandler) handleError(w http.ResponseWriter, req *http.Request, error int, useCompression bool) {
	Debug("+HandleError")
	req.Header.Del(HeaderIfModifiedSince)

	if errorFile := this.findErrorFile(error); errorFile != "" {

		req.URL.Path = errorFile
		if fc, err := this.FileAccessor.GetFile(req, this.Resource, useCompression); err == nil {
			this.writeFile(w, req, fc)
		} else {
			w.WriteHeader(error)
		}
	} else {
		w.WriteHeader(error)
	}
	Debug("-HandleError")
}

// ------------------------------------------------------------------------------------------------------------------------
// Non-Exported functions
// ------------------------------------------------------------------------------------------------------------------------

// writeFile writes file contents to http.ResponseWriter
//
// It works by attempting to combine ServerResource.Path (from config) with the request path
// + defaulting extensions or files if they're missing (also from config). If everythings OK
// it should return 'OK' (200) or 'Not Modified' (304), otherwise its an error code
func (this *FSHandler) writeFile(w http.ResponseWriter, req *http.Request, content *FileContent) {
	
	fileInfo := content.FileInfo

	// Set content-type based on extension
	setContentTypeHeader(w, fileInfo)
	
	// If client already has file then return not modified, no need to write body
	if !isModifiedSince(req, content.AbsolutePath, content.FileInfo) {
		Debug("+writeFile - File not modified")
		w.WriteHeader(http.StatusNotModified)
		return

	// Set cache headers so clients with subsequently send If-Modified-Since header
	} else {
		w.Header()[HeaderExpires] = []string{ ValueExpires }
		w.Header()[HeaderCacheControl] = []string{ ValueCacheControl }
		w.Header()[HeaderLastModified] = []string{ fileInfo.ModTime().In(GMTLoc).Format(time.RFC1123) }
	}

	// Check if we should be using compression or not + set header
	if content.Compression {
		Debug("+writeFile - Using compression")
		w.Header()[HeaderContentEncoding] = []string{CompressionGzip}
	}

	// Write response body
	Debug("Found file: " + content.AbsolutePath)
	Debug("File size: " + strconv.Itoa(len(content.Data)))
	if _, writeErr := w.Write(content.Data); writeErr != nil {
		this.handleError(w, req, int(http.StatusInternalServerError), content.Compression)
		return
	}
}

// findErrorFile attempts to return the path of an error file matching the error code
//
// It runs through the Regex in RequestContext.ErrorMap to see if it can find a match.
// Otherwise it returns an empty string and an error
func (this *FSHandler) findErrorFile(error int) (string) {
	// See if we have a specific file for the error by running through error map
	errStr := strconv.Itoa(error)
	for _, errorMapping := range this.ErrorMappings {

		// If we have a match...
		if errorMapping.Pattern.MatchString(errStr) {
			return errorMapping.Path
		}
	}
	return ""
}

// shouldUseCompression detects whether we should consider compressing the response or not
//
// It detects whether the client has specified they can handle gzip and whether compression has been specified
// in the config file. Whether compression is actually used depends on FileSystemLoader as it won't attempt
// compression if the file turns out to be an image
func (this *FSHandler) shouldUseCompression(req *http.Request) bool {
	compressionTypes, acceptsCompression := req.Header[HeaderAcceptEncoding]
	return this.Resource.Compression && acceptsCompression && containsInArray(compressionTypes, CompressionGzip)
}

// isModifiedSince checks to see if the file has changed since the client last requested
//
// Checks for 'If-Modified-Since' header and compares timestamp against current
// timestamp of file. Returns true if the files timestamp is different to the one the
// client sent along
func isModifiedSince(req *http.Request, url string, fi os.FileInfo) bool {
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

// containsInArray is a utility function to check if a string is contained in any of the array items
//
// Currently used to see if the client accepts gzip encoding
func containsInArray(vals []string, str string) bool {
	for _, val := range vals {
		if strings.Index(val, str) != -1 {
			return true
		}
	}
	return false
}

// setContentTypeHeader sets the 'content-type' header of the http response based on the file extension
func setContentTypeHeader(w http.ResponseWriter, fileInfo os.FileInfo) {
	for key, val := range mimeMap {
		if strings.HasSuffix(fileInfo.Name(), key) {
			w.Header()[HeaderContentType] = []string{val}
			return
		}
	}
	w.Header()[HeaderContentType] = []string{PlainTextMimeType}
}