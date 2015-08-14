package reverseproxy

import (
	"net/http"
	"fmt"
	"strings"
	"regexp"
)

const (
	FileSystem = "file_system"
	UnixSocket = "unix_socket"
	HttpSocket = "http_socket"
)

type HandlerFunc func(w http.ResponseWriter, req *http.Request, res *ServerResource)

type ServerHandler struct {
	DefaultMappings []PathMapping
	HostMappings map[string][]PathMapping
}

type PathMapping struct {
	Pattern *regexp.Regexp
	Handler HandlerFunc
	Resource *ServerResource
}

func (sh *ServerHandler) HostHandler(w http.ResponseWriter, req *http.Request) {
	// Remove port if required
	host := req.Host
	colonIndex := strings.Index(host, ":")
	if colonIndex != -1 {
		host = host[:colonIndex]
	}

	// Get correct ServerBlock
	mappings, OK := sh.HostMappings[host]
	if !OK {
		mappings = sh.DefaultMappings
	}

	// Now we need to match path
	mapping := matchMapping(mappings, req)
	if mapping != nil {
		mapping.Handler(w, req, mapping.Resource)
	} else {
		panic("Implement 404 handler")
	}
}

func (sh *ServerHandler) UnixSocketHandler(w http.ResponseWriter, req *http.Request, res *ServerResource) {
	fmt.Fprintf(w, "Hello, UnixSocketHandler")
}

func (sh *ServerHandler) HTTPSocketHandler(w http.ResponseWriter, req *http.Request, res *ServerResource) {
	fmt.Fprintf(w, "Hello, HTTPSocketHandler")
}

func matchMapping(mappings []PathMapping, req *http.Request) *PathMapping {
	for _, mapping := range mappings {
		if mapping.Pattern.MatchString(req.URL.Path) {
			return &mapping
		}
	}
	return nil
}

func listenAndServe(handle *ServerHandler) {
	http.ListenAndServe(":7890", nil)
}

func createServerHandler(blocks []ServerBlock) (*ServerHandler) {
	sh := ServerHandler { HostMappings: make(map[string][]PathMapping) }
	i := 0

	for index, sb := range blocks {

		pathMappings := make([]PathMapping, 0)

		// Run through paths and create regex
		for _, resource := range sb.Content {
			resource.Init()
			
			re, err := regexp.Compile(resource.Match)
			if err != nil {
				panic(err)
			}

			var p PathMapping
			switch resource.Type {
			case FileSystem:
				p = PathMapping {Pattern: re, Handler: sh.FileHandler, Resource: &resource}
			case UnixSocket:
				p = PathMapping {Pattern: re, Handler: sh.UnixSocketHandler, Resource: &resource}
			case HttpSocket:
				p = PathMapping {Pattern: re, Handler: sh.HTTPSocketHandler, Resource: &resource}
			default:
				panic(fmt.Sprintf("Unknown handler Type: %s", resource.Type))
			}

			// _ = to stop the error 'evaluated but not used'
			pathMappings = append(pathMappings, p)
		}

		// Run through hostnames and create hashmap (TODO - probably better with trie here)
		for _, host := range sb.Hostnames {
			sh.HostMappings[host] = pathMappings
			if(host == "default") {
				i = index
			}
		}
	}
	sh.DefaultMappings = sh.HostMappings[blocks[i].Hostnames[0]]
	return &sh
}

func StartServerAsync(serverBlocks []ServerBlock) {
	sh := createServerHandler(serverBlocks)

	http.HandleFunc("/", sh.HostHandler)
	go listenAndServe(sh)
}