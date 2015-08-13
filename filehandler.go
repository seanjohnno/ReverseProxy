package reverseproxy

import (
	"os"
	"bytes"
	"io"
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

// Response header + values for type of content returned from server
const (
	ContentType 			= "Content-Type"
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

// HTML extension. Files requested without extension are assume to be .html 
const(
	HtmlExtension 			= ".html"
)

const(
	HttpNotModified			= 304
)

var (
	GMTLoc, GMTErr = time.LoadLocation("GMT")
)

// FileHandler combines "path" in server config block with the request path and attempts to fetch the file.
// It'll attempt to use gzip compression if the client accepts it. Stores the file in memory if the cache
// limit allows and also uses caching headers + If-Modified-Since so content doesn't need to be served
// again
func (sh *ServerHandler) FileHandler(w http.ResponseWriter, req *http.Request, res *ServerResource) {

	url := createUrl(req, res)

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

	// Check if we should be using compression or not
	compressionTypes, acceptsCompression := req.Header[HeaderAcceptEncoding]
	useCompression := res.AcceptEncoding && acceptsCompression && containsInArray(compressionTypes, CompressionGzip)

	// TODO - Only get here if its not present in cache

	// Buffer to hold file content
	var fileReader io.ReadCloser
	f, err := os.Open(*url)
	if err != nil {
		panic("Implement 404 handler")
	}

	// Use compression
	if useCompression {
		compressionReader, err := gzip.NewReader(fileReader)
		if err != nil {
			panic("Implement 500 handler - Compression Error")
		}
		defer compressionReader.Close()
		fileReader = compressionReader

		w.Header()[HeaderContentEncoding] = []string{CompressionGzip}

	} else {
		fileReader = f
	}
	defer fileReader.Close()

	// Read all of file content
	fileContent, err := ioutil.ReadAll(fileReader)
	if err != nil {
		panic(err)
	}

	// TODO - Need to check returned int against size?
	_, writeErr := w.Write(fileContent)
	if writeErr != nil {
		panic("Implement 500 handler - Writer error")
	}
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
		if val == str {
			return true
		}
	}
	return false
}