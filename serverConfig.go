package reverseproxy


import (
	"encoding/json"
	"io"
	"os"
)

// ------------------------------------------------------------------------------------------------------------------------
// struct: ServerBlock
// ------------------------------------------------------------------------------------------------------------------------

// ServerBlock is the top level element used to contain groupings of Host and Content blocks
//
// Different ServerBlocks can contain different Hosts to provide virtual host functionality
type ServerBlock struct {

	// Hosts is used to match on the "Host" header passed in the HTTP request
	Hosts []Host

	// Content is used to match on the "Path" passed in the HTTP request
	Content []ServerResource

	// Default indicates that if theres no host matches then use this as the default
	Default bool
}

// ------------------------------------------------------------------------------------------------------------------------
// struct: Host
// ------------------------------------------------------------------------------------------------------------------------

// Host contains the Name (to match a request) and optional HTTPS info
type Host struct {

	// Host is used to match the "Host" header passed by the HTTP request
	Host string

	// CertFile is used to point to the location of a certificate for HTTPS (empty if http)
	CertFile string

	// KeyFile is used to point to the location of a key file for HTTPS (empty if http)
	KeyFile string

	// Indicates port to start/listen on
	Port int
}

// ------------------------------------------------------------------------------------------------------------------------
// struct: ServerResource
// ------------------------------------------------------------------------------------------------------------------------

// ServerResource matches a path from a HTTP request and controls what handler the request is sent to
type ServerResource struct {

	// Match is a regular expression which matches the path sent in the http request
	//
	// If its not matched then this resource won't be run - simples
	Match string

	// Type is the type of handler we want
	//
	// Right now we have a couple:
	// 
	//	file_system - Form an absolute path from 'Path' and the request path and return a file
	//	unix_socket - Direct the request to another service listening on a unix socket
	//	http_socket - Direct the request to another service listening on a http socket
	Type string

	// Path depends on Type but it'll indicate either a filesystem root or a socket address
	//
	// If type is file_system, Path is /var/www/somedomain and the request path is /static/index.html
	// Then we'll look for a file at /var/www/somedomain/static/index.html.
	// 
	// If type is *_socket then it should contain the uri:port to pass it to
	Path string

	// CacheStrategy is specified if we want to use in-memory caching
	Cache CacheStrategy

	// FileSystemConfig is only used if the Type is set to file_system
	//
	// Used to specify defaults if a full file path isn't specified
	FSDefaults FileSystemDefaults

	// Compression indiciates whether we want to return gzip'd responses
	Compression bool

	// Error provides a map to match http error codes to error pages so the user is served these instead
	//
	// The key is a regular expression so we could have 40[0-9]: /error/40x.html
	Error map[string]string
}

// ------------------------------------------------------------------------------------------------------------------------
// struct: CacheStrategy
// ------------------------------------------------------------------------------------------------------------------------

// CacheStrategy is used to configure the type of caching we want
type CacheStrategy struct {

	// CacheName is used when creating/accessing the cache
	//
	// It allows multiple ServerResource blocks to share the same cache if required
	Name string

	// Strategy indicates the caching algorithm used.
	//
	// Right now this can only be lru, empty if no cache required
	Strategy string

	// CacheLimit is the maximum size in bytes the cache is allowed to grow to
	Limit int
}

// ------------------------------------------------------------------------------------------------------------------------
// struct: FileSystemConfig
// ------------------------------------------------------------------------------------------------------------------------

// FileSystemDefaults is used to specify defaults if a full file path isn't specified
//
// It helps to create search engine friendly URLS
type FileSystemDefaults struct {

	// DefaultFiles allows us to specify some optional files to look for if only a path is specified
	//
	// For example if '/' is passed in the path we could have []string{ "index.html" } here so that
	// file is returned if no path is specified by the browser
	DefaultFiles []string

	// DefaultExtensions allows us to specify some optional extensions to try if no extension is specified
	//
	// This allows us to have search engine friend urls. For example, if '/index' is requested we
	// could have []string{ ".html" } here so /index.html is returned
	DefaultExtensions []string
}

// ------------------------------------------------------------------------------------------------------------------------
// Constructor Functions
// ------------------------------------------------------------------------------------------------------------------------

// LoadConfigFromFile parses and returns our []ServerBlock from the config file it's been passed
func LoadConfigFromFile(configLocation string) ([]ServerBlock, error) {
	file, err := os.Open(configLocation)

	if err != nil {
		return nil, err	
	}

	return LoadConfigFromReader(file)
}

// LoadConfigFromFile parses and returns our []ServerBlock from the Reader it's been passed
func LoadConfigFromReader(config io.Reader) ([]ServerBlock, error) {
	sb := make([]ServerBlock, 0)
	d := json.NewDecoder(config)
	decodeErr := d.Decode(&sb)

	if decodeErr != nil {
		panic(decodeErr)
	}

	return sb, decodeErr
}