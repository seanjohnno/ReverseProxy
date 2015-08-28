package reverseproxy

import (
	"testing"
	"github.com/seanjohnno/memcache"
	"io/ioutil"
	"os"
	"time"
)

const (
	TestFile = "./testfiles/hellotest.txt"
)

func TestFileMemCache(t *testing.T) {
	cache := memcache.CreateLRUCache(1024)
	fileInfo, _ := os.Stat(TestFile)
	accessor := NewFileCache(TestFile, fileInfo, false, cache)

	if fileContent := accessor.GetFile(); fileContent != nil {
		t.Error("File content should be nil")
	}

	// Lets load file and add into cache
	if fileContent, err := ioutil.ReadFile(TestFile); err == nil {
		accessor.PutFile(fileContent, fileInfo.ModTime())
	} else {
		t.Error("Couldn't load file")
	}

	// Retrieve it from cache
	fileInfo, _ = os.Stat(TestFile)
	accessor = NewFileCache(TestFile, fileInfo, false, cache)
	if fileContent := accessor.GetFile(); fileContent == nil {
		t.Error("File should be in cache!")
	}

	// Now lets test that if we're after a compressed version of the file, it won't return an uncompressed one
	fileInfo, _ = os.Stat(TestFile)
	accessor = NewFileCache(TestFile, fileInfo, true, cache)
	if fileContent := accessor.GetFile(); fileContent != nil {
		t.Error("File shouldn't be returned")
	}

	// Test that we can still access the original
	fileInfo, _ = os.Stat(TestFile)
	accessor = NewFileCache(TestFile, fileInfo, false, cache)
	if fileContent := accessor.GetFile(); fileContent == nil {
		t.Error("We should still have original")
	}

	// Check that if we modify the file then the original cached object is invalid
	if file, err := os.OpenFile(TestFile, os.O_WRONLY, os.ModePerm); err == nil {
		if _, err := file.WriteString(time.Now().String()); err == nil {
				file.Close()

				// Now file is modified it should blow away original cache
				fileInfo, _ = os.Stat(TestFile)
				accessor = NewFileCache(TestFile, fileInfo, false, cache)
				if fileContent := accessor.GetFile(); fileContent != nil {
					t.Error("File should have been removed")
				}

		} else {
			t.Error("Error writing to file")
		}
	} else {
		t.Error("Error opening file for writing")
	}



	// Check that if the cache becomes full then files start being removed from the cache
}