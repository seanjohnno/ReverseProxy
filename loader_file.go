package reverseproxy

import (
	"errors"
	"os"
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"strings"
	"net/http"
)

const (
	MimeTextBased		= "text"
	PlainTextMimeType	= "text/plain"
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

// ------------------------------------------------------------------------------------------------------------------------
// Exported functions
// ------------------------------------------------------------------------------------------------------------------------

type FileRetriever interface {
	GetFile(req *http.Request, Resource *ServerResource, compression bool) (*FileContent, error)
}


type FileContent struct {

	// FileInfo is the FileInfo object at the time of cache
	FileInfo os.FileInfo

	// AbsolutePath is the absolute filepath to the file
	AbsolutePath string

	// Data is the file content, possibly compressed
	Data []byte
	
	// Compression indicates if Data has been compressed
	Compression bool

	// IgnoreCompression indicates whether we should ignore a compression request
	IgnoreCompression bool

	// MimeType is the mime to return to the client
	MimeType string
}

// Size is used to tell the cache how big this item is in bytes
//
// Implementing the CacheItem interface
func (this *FileContent) Size() int {
	return len(this.Data)
}

type FileSystemLoader struct {

}

func (this *FileSystemLoader) GetFile(req *http.Request, resource *ServerResource, compression bool) (*FileContent, error) {
	if fi, absolutePath := this.LocateFile(req.URL.Path, resource); fi != nil {
		
		// Get mimetype and figure out whether we should ignore compression flag
		mimeType := getContentTypeHeader(fi)
		ignoreCompression := !strings.HasPrefix(mimeType, MimeTextBased)
		
		if ignoreCompression {
			compression = false
		}

		if data, err := this.ReadFile(absolutePath, compression); err == nil {	
			return &FileContent{ fi, absolutePath, data, compression, ignoreCompression, mimeType }, nil
		} else {
			return nil, err
		}
	} else {
		return nil, errors.New("Unable to locate file")
	}
}

func (this *FileSystemLoader) LocateFile(requestPath string, res *ServerResource) (os.FileInfo, string) {
	filePath :=  res.Path + requestPath

	// If we finish in a slash then we're a directory and we need a default file
	if strings.HasSuffix(requestPath, "/") {
		// Run through all default files supplied in the config
		if fullPath, fileInfo := this.FindFileByAppending(filePath, res.FSDefaults.DefaultFiles); fileInfo != nil {
			return fileInfo, fullPath
		}

	// No extension so lets try and append the ones specified as default
	} else if !strings.Contains(requestPath, ".") {
		// Run through all default extensions supplied in the config
		if fullPath, fileInfo := this.FindFileByAppending(filePath, res.FSDefaults.DefaultExtensions); fileInfo != nil {
			return fileInfo, fullPath
		}

	// Check file
	} else if f, err := os.Stat(filePath); err == nil {
		return f, filePath
	}

	return nil, requestPath
}

func (this *FileSystemLoader) ReadFile(absolutePath string, compression bool) ([]byte, error) {
	if fileContent, err := ioutil.ReadFile(absolutePath); err == nil {
		
		// If compression flag is set then compress and assign to fileContent
		if compression {
			buf := bytes.NewBuffer( make([]byte, 0) )
	
			compressionWriter := gzip.NewWriter(buf)
			_, err := compressionWriter.Write(fileContent)
			compressionWriter.Close()
			if err != nil {
				return nil, err
			} 
			
			fileContent = buf.Bytes()
		}

		// Add cache object
		return fileContent, nil
	} else {
		return nil, err
	}
}


// findFileByAppending loops through slice appending to the path until it finds a file that exists
//
// Returned FileInfo will be nil if it can't find any that exist
func (this *FileSystemLoader) FindFileByAppending(filePath string, appendSlice []string) (string, os.FileInfo) {
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

// setContentTypeHeader sets the 'content-type' header of the http response based on the file extension
func getContentTypeHeader(fileInfo os.FileInfo) string {
	for key, val := range mimeMap {
		if strings.HasSuffix(fileInfo.Name(), key) {
			return val
		}
	}
	return PlainTextMimeType
}