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
	PlainTextMimeType		= "text/plain"
)

var (
	// mimeMap maps file extensions to content types - TODO - needs to be expanded / perhaps read from a config file(?)
	mimeMap = map[string]string {
		".html": "text/html",
		".css": "text/css",	
		".js": "text/javascript",
		".ico": "image/x-icon",
		".jpg": "image/jpeg",
		".jpeg": "image/jpeg",
		".png": "image/png",
		".gif": "image/gif",
	}
)

var (
	// GMTLoc is a GTM Time struct, server needs to return GMT dates
	GMTLoc, _ = time.LoadLocation("GMT")
)

// ------------------------------------------------------------------------------------------------------------------------
// Exported functions
// ------------------------------------------------------------------------------------------------------------------------

// HandleFS write files to response body
//
// It works by attempting to combine ServerResource.Path (from config) with the request path
// + defaulting extensions or files if they're missing (also from config)
func HandlerFS(w http.ResponseWriter, req *http.Request, context *RequestContext) {

	Debug("+HandlerFS - Path: " + req.URL.Path)

	// Combine fs path + request path to create absolute path
	path, fileInfo := findFile(req.URL.Path, context.Resource)
	
	// If file hasn't been found then write error page or error response
	if fileInfo == nil {
		HandleError(w, req, context, http.StatusNotFound)
		return
	}

	// Try to write our file to http.ResponseWriter
	if retCode := writeFile(w, req, context, fileInfo, path); !(retCode == http.StatusOK || retCode == http.StatusNotModified) {
		
		// If error writing then write error page or error response
		HandleError(w, req, context, retCode)
	}
}

// handleError will attempt to serve an error page instead of a status code
//
// If it has a handler for 
func HandleError(w http.ResponseWriter, req *http.Request, context *RequestContext, error int) {

	// If we can't find a mapping for the error:page then just write status code
	if errorPath, fileInfo := findErrorFile(context, int(error)); fileInfo == nil {
		w.WriteHeader(error)

	// If we can find a mapping...
	} else {
		// Remove If-Modified-Since as it doesn't relate to this error page
		req.Header.Del(HeaderIfModifiedSince)

		// Attempt to write error page, if theres an error doing this then just write status code
		if retCode := writeFile(w, req, context, fileInfo, errorPath); retCode != http.StatusOK {
			w.WriteHeader(error)
		}
	}
}

// ------------------------------------------------------------------------------------------------------------------------
// Non-Exported functions
// ------------------------------------------------------------------------------------------------------------------------

// writeFile writes file contents to http.ResponseWriter
//
// It works by attempting to combine ServerResource.Path (from config) with the request path
// + defaulting extensions or files if they're missing (also from config). If everythings OK
// it should return 'OK' (200) or 'Not Modified' (304), otherwise its an error code
func writeFile(w http.ResponseWriter, req *http.Request, context *RequestContext, fileInfo os.FileInfo, path string) (int) {
	// Set content-type based on extension
	setContentTypeHeader(w, fileInfo)
	
	// If client already has file then return not modified, no need to write body
	if !isModifiedSince(req, path, fileInfo) {
		Debug("+writeFile - File not modified")
		w.WriteHeader(http.StatusNotModified)
		return http.StatusNotModified

	// Set cache headers so clients with subsequently send If-Modified-Since header
	} else {
		w.Header()[HeaderExpires] = []string{ ValueExpires }
		w.Header()[HeaderCacheControl] = []string{ ValueCacheControl }
		w.Header()[HeaderLastModified] = []string{ fileInfo.ModTime().In(GMTLoc).Format(time.RFC1123) }
	}

	// Check if we should be using compression or not + set header
	compressionTypes, acceptsCompression := req.Header[HeaderAcceptEncoding]
	useCompression := context.Resource.Compression && acceptsCompression && containsInArray(compressionTypes, CompressionGzip)
	if useCompression {
		Debug("+writeFile - Using compression")
		w.Header()[HeaderContentEncoding] = []string{CompressionGzip}
	}

	var fileContent []byte
	fileCache := NewFileCache(path, fileInfo, useCompression, context.Cache)

	// If we have cache then we can just set data here
	if fileContent = fileCache.GetFile(); fileContent == nil {
		Debug("+writeFile - Loading from filesystem")

		// Otherwise we load it from the filesystem
		var fileErr error
		if fileContent, fileErr = loadResourceFromFS(path, useCompression); fileErr != nil {
			return http.StatusInternalServerError
		}

		// Add it to the cache
		fileCache.PutFile(fileContent, fileInfo.ModTime())
	} else {
		Debug("+writeFile - Serving from cache")
	}

	// Write response body
	if _, writeErr := w.Write(fileContent); writeErr != nil {
		return http.StatusInternalServerError
	}

	return http.StatusOK
}

// findErrorFile attempts to return the path of an error file matching the error code
//
// It runs through the Regex in RequestContext.ErrorMap to see if it can find a match.
// Otherwise it returns an empty string and an error
func findErrorFile(context *RequestContext, error int) (string, os.FileInfo) {
	// See if we have a specific file for the error by running through error map
	errStr := strconv.Itoa(error)
	for _, errorMapping := range context.ErrorMappings {

		// If we have a match...
		if errorMapping.Pattern.MatchString(errStr) {

			// See if we can load file
			if path, fileInfo := findFile(errorMapping.Path, context.Resource); fileInfo != nil {
				return path, fileInfo
			}
		}
	}
	return "", nil
}

// findFile takes ServerResource.Path + request.URL.Path + extension OR file detaults and tries to locate a file
//
// Returned fileInfo will be nil if no valid file could be found
func findFile(requestPath string, res *ServerResource) (string, os.FileInfo) {

	filePath :=  res.Path + requestPath

	// If we finish in a slash then we're a directory and we need a default file
	if strings.HasSuffix(requestPath, "/") {
		// Run through all default files supplied in the config
		if fullPath, fileInfo := findFileByAppending(filePath, res.FSDefaults.DefaultFiles); fileInfo != nil {
			return fullPath, fileInfo
		}

	// No extension so lets try and append the ones specified as default
	} else if !strings.Contains(requestPath, ".") {
		// Run through all default extensions supplied in the config
		if fullPath, fileInfo := findFileByAppending(filePath, res.FSDefaults.DefaultExtensions); fileInfo != nil {
			return fullPath, fileInfo
		}
	}

	// We either have full path or we haven't found the file, try to load either way
	if f, err := os.Stat(filePath); err == nil {
		return filePath, f
	}

	return filePath, nil
}

// findFileByAppending loops through slice appending to the path until it finds a file that exists
//
// Returned FileInfo will be nil if it can't find any that exist
func findFileByAppending(filePath string, appendSlice []string) (string, os.FileInfo) {
	// Run through list specified in config
	for _, suffix := range appendSlice {
		fullPath := filePath + suffix

		if f, err := os.Stat(fullPath); err == nil {
			Debug("+findFileByAppending. Found file: " + fullPath)
			return fullPath, f
		}
		Debug("+findFileByAppending. No match for file: " + fullPath)
	}
	return "", nil
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

// loadResourceFromFS grabs a files content and compresses it if specified
func loadResourceFromFS(path string, useCompression bool) ([]byte, error) {
	// Buffer to hold file content
	f, err := os.Open(path)
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
