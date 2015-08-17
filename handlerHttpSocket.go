package reverseproxy

import (
	"net/http"
	"fmt"
)

func HandlerHttpSocket(w http.ResponseWriter, req *http.Request, context *RequestContext) {
	fmt.Fprintf(w, "Hello HttpSocket")
}
