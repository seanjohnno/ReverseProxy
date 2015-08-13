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
	Type, Path, Cachestrategy, Match string	
	Cachelimit int
	AcceptEncoding bool
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
