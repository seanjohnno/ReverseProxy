package reverseproxy

import (
	"net/http"
	"fmt"
)

type UnixHandler struct {

	// FSHandler contains ServerResource & ErrorMappings map
	FSHandler
}

// NewHttpHandler returns an *NewHttpHandler
func NewUnixHandler(rsc *ServerResource, errorMappings []ErrorMapping) (*UnixHandler) {
	
	// FileAccessor handles null cache
	return &UnixHandler{ FSHandler: *NewFSHandler( rsc, errorMappings, nil ) }
}


func (this *UnixHandler) HandleRequest(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "Hello UnixSocket")
}
