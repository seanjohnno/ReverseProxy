package reverseproxy


import (
	"encoding/json"
	"io"
	"os"
)

// ----------------------------------------

type ServerBlock struct {
	Hostnames []string
	Content []ServerResource
}

type ServerResource struct {
	Type, Path, CacheStrategy, Match string	
	Cachelimit int
	Compression bool
	Cache Cache
}

func (this *ServerResource) Init() {
	switch this.CacheStrategy {
	case "lru":
		if this.Cachelimit <= 0 {
			panic("Need to set CacheLimit to a positive integer")
		}
		this.Cache = CreateLRUCache(this.Cachelimit)
	case "":
		// Do nothing - No cache strategy
	default:
		panic("Unknown cache strategy")
	}
}

// ----------------------------------------

func LoadConfigFromFile(configLocation string) ([]ServerBlock, error) {
	file, err := os.Open(configLocation)

	if err != nil {
		return nil, err	
	}

	return LoadConfig(file)
}


func LoadConfig(config io.Reader) ([]ServerBlock, error) {
	sb := make([]ServerBlock, 0)
	d := json.NewDecoder(config)
	decodeErr := d.Decode(&sb)

	if decodeErr != nil {
		panic(decodeErr)
	}

	return sb, decodeErr
}
