package reverseproxy

import (
	"testing"
	"fmt"
	"net/http"
	"os"
	"github.com/seanjohnno/memcache"
	"strconv"
	"time"
)

var (
	BaseUrl = ""
)

// ------------------------------------------------------------------------------------------------------------------------
// Testing cache_builder.go
// ------------------------------------------------------------------------------------------------------------------------

func TestCacheBuilder(t *testing.T) {
	cb := CreateCacheBuilder()

	//  Test a zero size cache fails
	if cache, err := cb.CreateCache("", "", 0); cache != nil || err == nil {
		t.Error("Cache creation should have failed with zero size")
	}

	//  Test unknown cache
	if cache, err := cb.CreateCache("", "JohnnoSmash", 50); cache != nil || err == nil {
		t.Error("Cache creation should have failed with zero size")
	}	

	// Test we can create a default cache
	defaultCache, err := cb.CreateCache("", LRUCache, 50)
	if defaultCache == nil || err != nil {
		t.Error("Cache creation should have succeeded")
	}

	// Test we can create two separate default caches
	defaultCacheTwo, err := cb.CreateCache("", LRUCache, 50)
	if defaultCacheTwo == nil || err != nil {
		t.Error("2. Cache creation should have succeeded")
	} else if(fmt.Sprintf("%p", defaultCache) == fmt.Sprintf("%p", defaultCacheTwo)) {
		t.Error("Default cache pointers should point to different cache objects")
	}

	// Test we can create a named cache object
	namedCache, err := cb.CreateCache("Named", LRUCache, 50)
	if namedCache == nil || err != nil {
		t.Error("Named cache creation should have succeeded")
	}

	// Test we can access the same cache object via its name
	namedCacheTwo, err := cb.CreateCache("Named", LRUCache, 50)
	if namedCacheTwo == nil || err != nil {
		t.Error("2. Named cache creation should have succeeded")
	} else if(fmt.Sprintf("%p", namedCache) != fmt.Sprintf("%p", namedCacheTwo)) {
		t.Error("Named cache pointers should point to the same cache object")
	}
}

// ------------------------------------------------------------------------------------------------------------------------
// Testing handler_filesystem.go
// ------------------------------------------------------------------------------------------------------------------------

func TestFileSystemHandler(t *testing.T) {

	if workingDir, err := os.Getwd(); err != nil {
		t.Error("Couldn't get working directory")
	
	} else {
		BaseUrl = "http://localhost"

		// Create server resource object that points at our test files directory
		sr := &ServerResource {
			Match: "/", Type: "file_system", Path: workingDir + "/testfiles", 
			Cache: CacheStrategy{ Name: "", Strategy: "lru", Limit: 1024}, 
			FSDefaults: FileSystemDefaults{ DefaultFiles: []string{ "index.html", "hello.html" }, DefaultExtensions: []string{ ".html", ".css" }}, 
			Compression: false, 
			Error: []ErrorRedirect { ErrorRedirect{ Match:"404", Path:"/404.txt" } },
		}
		
		// Create cache builder
		cb := &DummyCacheBuilder{}

		// Create our file handler
		fsHandler := NewFSHandler(sr, CreateErrorMapping(*sr), cb)

		var r *DummyResponseWriter

		// Test normal request works
		if r = HttpGet("/index.html", fsHandler, t); r == nil || r.RespCode != 200 || r.Data == nil || len(r.Data) == 0 {
			t.Error("/index.html request failed")
		}

		// Test no extension request works
		if r = HttpGet("/index", fsHandler, t); r == nil || r.RespCode != 200 || r.Data == nil || len(r.Data) == 0 {
			t.Error("/index request failed, should find /index.html")
		}

		// Test second extension request works
		if r = HttpGet("/test", fsHandler, t); r == nil || r.RespCode != 200 || r.Data == nil || len(r.Data) == 0 {
			t.Error("/test request failed, should find /test.css")
		}

		// Test css mime
		if contentType, ok := r.Headers["Content-Type"]; !ok || len(contentType) == 0 || contentType[0] != "text/css" {
			t.Error("Should have content type of text/html")
		}
	
		// Test no file request works
		if r = HttpGet("/", fsHandler, t); r == nil || r.RespCode != 200 || r.Data == nil || len(r.Data) == 0 {
			t.Error("/ request failed, should find /index.html")
		}

		// Test no file in subdir works
		if r = HttpGet("/subdir/", fsHandler, t); r == nil || r.RespCode != 200 || r.Data == nil || len(r.Data) == 0 {
			t.Error("/subdir/ request failed, should find /subdir/hello.html")
		}

		// Test mime type
		if contentType, ok := r.Headers["Content-Type"]; !ok || len(contentType) == 0 || contentType[0] != "text/html" {
			t.Error("Should have content type of text/html")
		}

		// Test not-modified works
		lastModified := r.Headers["Last-Modified"]
		if r = HttpGetWithHeaders("/subdir/", fsHandler, map[string][]string{ "If-Modified-Since": lastModified }, t); r == nil || r.RespCode != 304 || len(r.Data) > 0 {
			t.Error("Response should have been 304")
		}

		// Test cache works
		if cb.Cache.lastAddKey != "/subdir/" {
			t.Error("/subdir/ should have been cached")
		}

		// Test we're not compressing without specifying 'Accept-Encoding'
		sr.Compression = true
		if r = HttpGet("/subdir/", fsHandler, t); r == nil || r.RespCode != 200 || r.Data == nil || len(r.Data) == 0 {
			t.Error("/subdir/ response should have been 200")
		} else if _, ok := r.Headers["Content-Encoding"]; ok {
			t.Error("Shouldn't have encoded when we haven't specified 'Accept-Encoding'")
		}

		// Test compression works
		contentEncoding := []string{ "gzip" }
		if r = HttpGetWithHeaders("/subdir/", fsHandler, map[string][]string{ "Accept-Encoding": contentEncoding }, t); r == nil || r.RespCode != 200 || len(r.Data) == 0 {
			t.Error("/subdir/ request should return 200 with accept-encoding")
		} else if ce, ok := r.Headers["Content-Encoding"]; !ok || len(ce) == 0 || ce[0] != "gzip" {
			t.Error("Content encoding should be gzip")
		}

		// Test compression when multiple encoding specified
		contentEncoding = []string{ "deflate", "gzip" }
		if r = HttpGetWithHeaders("/subdir/", fsHandler, map[string][]string{ "Accept-Encoding": contentEncoding }, t); r == nil || r.RespCode != 200 || len(r.Data) == 0 {
			t.Error("/subdir/ request should return 200 with accept-encoding")
		} else if ce, ok := r.Headers["Content-Encoding"]; !ok || len(ce) == 0 || ce[0] != "gzip" {
			t.Error("Content encoding should be gzip")
		}

		// Test no-compression on images
		if r = HttpGetWithHeaders("/gopher.png", fsHandler, map[string][]string{ "Accept-Encoding": contentEncoding }, t); r == nil || r.RespCode != 200 || len(r.Data) == 0 {
			t.Error("/gopher.png request should return 200 with accept-encoding")
		} else if _, ok := r.Headers["Content-Encoding"]; ok {
			t.Error("We shouldn't be encoding image types")
		}

		// Error mapping test
		if r = HttpGet("/doesntexist.html", fsHandler, t); r == nil || r.RespCode != 200 || r.Data == nil || len(r.Data) == 0 {
			t.Error("/doesntexist.html should have returned error file, returned", strconv.Itoa(r.RespCode))
		} else if string(r.Data) != "404" {
			t.Error("/doesntexist.html should be returning the error file /404.txt")
		}

		// Test a regex and match order
		sr.Error = []ErrorRedirect { ErrorRedirect{ Match:"40[0-9]", Path:"/40x.txt" }, ErrorRedirect{ Match:"404", Path:"/404.txt" } }
		fsHandler.ErrorMappings = CreateErrorMapping(*sr)
		if r = HttpGet("/doesntexist.html", fsHandler, t); r == nil || r.RespCode != 200 || r.Data == nil || len(r.Data) == 0 {
			t.Error("/doesntexist.html should have returned error file, returned", strconv.Itoa(r.RespCode))
		} else if string(r.Data) != "40x" {
			t.Error("/doesntexist.html should be returning the error file /40x.txt")
		}
	}
}

// ------------------------------------------------------------------------------------------------------------------------
// Test HttpHandler
// ------------------------------------------------------------------------------------------------------------------------

func TestHTTPHandler(t *testing.T) {

	BaseUrl = "http://localhost:7890"

	// Start http handler, it'll return the path that its passed
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.URL.Path))
	})
	go func() {
		http.ListenAndServe(":7890", nil)
	}()

	// Wait for http server to come alive
	for i := 0; ; i++ {
		if i == 5 {
			t.Error("Unable to start http server, can't complete http tests")
			return
		
		} else if resp, err := http.Get("http://localhost:7890"); err == nil && resp.StatusCode == 200 {
			break
		}
		time.Sleep(time.Second)
	}

	// Create http handler
	sr := &ServerResource {
			Match: "/", Type: "http_socket", Path: "http://localhost:7890",
			Error: []ErrorRedirect { ErrorRedirect{ Match:"40[0-9]", Path:"/40x.txt" }, ErrorRedirect{ Match:"404", Path:"/404.txt" } },
	}
	httpHandler := NewHttpHandler(sr, CreateErrorMapping(*sr))

	// Check that our http request is passed to our handler
	var r *DummyResponseWriter
	if r = HttpGet("/heyhey", httpHandler, t); r == nil || r.RespCode != 200 || r.Data == nil || len(r.Data) == 0 || string(r.Data) != "/heyhey" {
		t.Error("Data should be /heyhey")
	}
}

// ------------------------------------------------------------------------------------------------------------------------
// Test Utility/Dummy classes
// ------------------------------------------------------------------------------------------------------------------------

// DummyResponseWriter

type DummyResponseWriter struct {
	Headers http.Header
	Data []byte
	RespCode int
}

func CreateDummyResponseWriter() *DummyResponseWriter {
	return &DummyResponseWriter { Headers: make(map[string][]string), Data: nil, RespCode: 200 }
}

func (this *DummyResponseWriter) Header() http.Header {
	return this.Headers
}

func (this *DummyResponseWriter) Write(newData []byte) (int, error) {
	this.Data = append(this.Data, newData...)
	return len(newData), nil
}

func (this *DummyResponseWriter) WriteHeader(respCode int) {
	this.RespCode = respCode
}

// DummyCacheBuilder

type DummyCacheBuilder struct {
	Cache *DummyCache
}

func (this *DummyCacheBuilder) CreateCache(cacheName string, cacheType string, cacheLimit int) (memcache.Cache, error) {
	this.Cache = &DummyCache{}
	return this.Cache, nil
}

// DummyCache

type DummyCache struct {
	lastAddKey string
	lastAddVal memcache.CacheItem
	lastGetKey string
	lastRemoveKey string
}

func (this *DummyCache) Add(key string, val memcache.CacheItem) error {
	this.lastAddKey = key
	this.lastAddVal = val
	return nil
}

func (this *DummyCache) Get(key string) (memcache.CacheItem, bool) {
	this.lastGetKey = key
	return nil, false
}

func (this *DummyCache) Remove(key string) {
	this.lastRemoveKey = key
}

// Utility http request functions

func HttpGet(path string, handler RequestHandler, t *testing.T) (*DummyResponseWriter) {
	return HttpGetWithHeaders(path, handler, nil, t)
}

func HttpGetWithHeaders(path string, handler RequestHandler, headers map[string][]string, t *testing.T) (*DummyResponseWriter) {
	if rq, err := http.NewRequest("GET", BaseUrl + path, nil); err != nil {
		t.Error("Failed to create request " + path)
		return nil
	} else {
		// Add additional headers
		if headers != nil {
			for k, v := range headers {
				rq.Header[k] = v
			}
		}

		rw := CreateDummyResponseWriter()
		handler.HandleRequest(rw, rq)
		return rw
	}
}



// Write test for server.go

// Write test for loader_file.go

// Write test for loader_cache.go

// Write test for handler_filesystem

// Write test for handler_http_socket

// Write test for handler_unix_socket