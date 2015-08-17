package reverseproxy

import (
	"net/http"
	"fmt"
)

func HandlerUnixSocket(w http.ResponseWriter, req *http.Request, context *RequestContext) {
	fmt.Fprintf(w, "Hello UnixSocket")
}
