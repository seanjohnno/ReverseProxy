package reverseproxy

import (
	"net/http"
	"fmt"
	"strings"
	"regexp"
	"strconv"
)

// Handler types. Known 'type' to use inside content block
const (
	FileSystem = "file_system"
	UnixSocket = "unix_socket"
	HttpSocket = "http_socket"
)

// ------------------------------------------------------------------------------------------------------------------------
// interface: Handler
// ------------------------------------------------------------------------------------------------------------------------

// Handler is the function to implement to handle http requests
type HandlerFunc func(w http.ResponseWriter, req *http.Request, context *RequestContext)

// ------------------------------------------------------------------------------------------------------------------------
// struct: ServerHandler
// ------------------------------------------------------------------------------------------------------------------------

// ServerHandler is used to normalise []ServerBlock returned from config
//
// It's used to first match on a 'Host' and then match on the path requested
type ServerHandler struct {

	// HostMappings is used to grab the []PathMapping slice based on the host passed into the request
	HostMappings map[string][]PathMapping
	
	// DefaultMappings holds a (ptr to) slice labelled as the default if no Host match is found
	DefaultMappings []PathMapping
}

// HostHandler takes a request and passes it 
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
		mapping.Handler(w, req, mapping.Context)
	} else {
		panic("Implement 404 handler")
	}
}

// ------------------------------------------------------------------------------------------------------------------------
// struct: PathMapping
// ------------------------------------------------------------------------------------------------------------------------

// PathMapping is used to match a URL request path and pass the request to the correct handler
type PathMapping struct {

	// Pattern is a regex expression used to see if the request path matches
	Pattern *regexp.Regexp

	// Handler is the function called (to write the response) if Pattern matches
	Handler HandlerFunc

	// Context is used to give the RequestHandler function some context on why it was called + access to cache
	Context *RequestContext
}

// ------------------------------------------------------------------------------------------------------------------------
// struct: ErrorMapping
// ------------------------------------------------------------------------------------------------------------------------

// ErrorMapping is used to match a http error to an error page
type ErrorMapping struct {

	// Pattern is a regex expression used to see if the error matches
	Pattern *regexp.Regexp

	// Path is used to point to a (relative) path on the filesystem
	Path string
}

// ------------------------------------------------------------------------------------------------------------------------
// struct: RequestContext
// ------------------------------------------------------------------------------------------------------------------------

// RequestContext is used to give request handlers context on why they were called and access to resources
type RequestContext struct {

	// Resource is used to give the RequestHandler function some context on why it was called
	//
	// This is so it knows how to seek a file or what are the connection details for a socket. Whether to use compression
	// etc
	Resource *ServerResource

	// Cache implementation if specified in config
	Cache Cache

	// ErrorMap is used when an error occurs and we want to display an error page rather than send an error code
	ErrorMappings []ErrorMapping
}

// CreateRequestContext creates and initialises a RequestContext
//
// Checks existing cacheMap as cache objects can be shared
func CreateRequestContext(resource *ServerResource, cacheMap map[string]Cache) (*RequestContext) {
	rc := &RequestContext{ Resource: resource}
	
	// We have CacheName so we want to check if its already been created
	if resource.Cache.CacheName != "" {
		// It its present we can just assign
		if c, OK := cacheMap[resource.Cache.CacheName]; OK {
			rc.Cache = c

		// If its not present then create and add to hash
		} else {
			c = CreateCache(resource)
			cacheMap[resource.Cache.CacheName] = c
			rc.Cache = c
		}
	// No CacheName so we just create
	} else {
		rc.Cache = CreateCache(resource)
	}

	// Create a map of error codes to file locations
	if resource.Error != nil {
		em := make([]ErrorMapping, 0)
		
		for k, v := range resource.Error {
			re, err := regexp.Compile(k)
			if err != nil {
				panic(err)
			}
			em = append(em, ErrorMapping { Pattern: re, Path: v } )
		}

		rc.ErrorMappings = em
	}

	return rc
}

func CreateCache(rsc *ServerResource) Cache {
	switch rsc.Cache.Strategy {
		// TODO - Need to add a map to memcache
	case "lru":
		return CreateLRUCache(rsc.Cache.CacheLimit)
	case "":
		panic("You need to specify a cache strategy")
	default:
		panic("Unknown cache strategy")
	}
}

// ------------------------------------------------------------------------------------------------------------------------
// Non-exported functions
// ------------------------------------------------------------------------------------------------------------------------

// matchMapping runs through PathMappings and returns a single mapping if its regular expression matches the request URL.Path
func matchMapping(mappings []PathMapping, req *http.Request) *PathMapping {
	for _, mapping := range mappings {
		if mapping.Pattern.MatchString(req.URL.Path) {
			return &mapping
		}
	}
	return nil
}

// listenAndServe runs through server blocks and figures out what ports to listen on + whether its http or https
func listenAndServe(serverBlocks []ServerBlock) {

	portsServed := make(map[int]bool)
	tlsPort := -1

	// Loop through each host in each server block
	for _, serverBlock := range serverBlocks {
		for _, host := range serverBlock.Hosts {

			// ...we haven't so create port string
			strPort := strconv.Itoa(host.Port)

			// Using https
			if host.CertFile != "" && host.KeyFile != "" {
				// We've already called ListenAndServeTLS()
				if tlsPort != -1 {
					// ...and now we're trying to use it for another virtual host on a different port, this can't work
					if host.Port != tlsPort {
						panic("Already serving HTTPS on a different port, you can't do this")
					}
					
				} else {
					go http.ListenAndServeTLS(":" + strPort, host.CertFile, host.KeyFile, nil)
					tlsPort = host.Port
				}

			// Using http
			} else {
				// Check we've not already called ListenAndServe on this port...
				if _, present := portsServed[host.Port]; !present {
					go http.ListenAndServe(":" + strPort, nil)
					portsServed[host.Port] = true
				}
			}
		}
	}	
}

// createServerHandler runs through []ServerBlock and outputs ServerHandler which is used for routing http requests
func createServerHandler(blocks []ServerBlock) (*ServerHandler) {

	// Blocks can share cache objects so create map here
	cacheMap := make(map[string]Cache)

	// Create our ServerHandler to hold all host/path mappings
	sh := ServerHandler { HostMappings: make(map[string][]PathMapping) }
	defaultMapping := 0

	for index, sb := range blocks {
		pathMappings := make([]PathMapping, 0)

		if sb.Default {
			defaultMapping = index
		}

		// Run through paths and create regex for each
		for _, resource := range sb.Content {

			// Create context so we can pass ServerResource and cache into requests
			context := CreateRequestContext(&resource, cacheMap)

			// Create regex to match paths
			re, err := regexp.Compile(resource.Match)
			if err != nil {
				panic(err)
			}

			// Determine the type of handler and assign function ptr
			var p PathMapping
			switch resource.Type {
			case FileSystem:
				p = PathMapping {Pattern: re, Handler: HandlerFS, Context: context}
			case UnixSocket:
				p = PathMapping {Pattern: re, Handler: HandlerUnixSocket, Context: context}
			case HttpSocket:
				p = PathMapping {Pattern: re, Handler: HandlerHttpSocket, Context: context}
			default:
				panic(fmt.Sprintf("Unknown handler Type: %s", resource.Type))
			}

			// Add mapping to our slice
			pathMappings = append(pathMappings, p)
		}

		// Run through hostnames and create hashmap (TODO - probably better with trie here)
		for _, host := range sb.Hosts {
			sh.HostMappings[host.Host] = pathMappings
		}
	}

	// Set the default mapping if there are no host matches
	sh.DefaultMappings = sh.HostMappings[blocks[defaultMapping].Hosts[0].Host]
	return &sh
}

// ------------------------------------------------------------------------------------------------------------------------
// Exported function
// ------------------------------------------------------------------------------------------------------------------------

// StartServerAync starts the server (doesn't block)
func StartServerAsync(serverBlocks []ServerBlock) {
	// Normalise config and create all handlers
	sh := createServerHandler(serverBlocks)

	// Match base path so everything is passed through our handler
	http.HandleFunc("/", sh.HostHandler)

	// Start listening on specified ports
	listenAndServe(serverBlocks)
}