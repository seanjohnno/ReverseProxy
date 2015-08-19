package reverseproxy

import (
	"net/http"
	"io"
	"bytes"
)

var (
	client = &http.Client{ }
)

const (
	BufferMax = 1024
)

func HandlerHttpSocket(w http.ResponseWriter, req *http.Request, context *RequestContext) {
	Debug("+HandlerHttpSocket - Loading from http connection")
	if status := handleSocket(w, req, context); !(status == http.StatusOK || status == http.StatusNotModified) {
		HandleError(w, req, context, status)
	}
}

func handleSocket(w http.ResponseWriter, req *http.Request, context *RequestContext) int {

	buf := bytes.Buffer { }
	buf.WriteString(context.Resource.Path)
	// buf.WriteString(req.URL.Path)
	
	// // Add query if we've got one
	// if len(req.URL.RawQuery) > 0 {
	// 	buf.WriteString("?")
	// 	buf.WriteString(req.URL.RawQuery)
	// }

	// // Add fragment if we've got one
	// if len(req.URL.Fragment) > 0 {
	// 	buf.WriteString("#")
	// 	buf.WriteString(req.URL.Fragment)
	// }

	Debug("+handleSocket - Method:", req.Method, "URL:", buf.String())

	// Create the request
	if newReq, err := http.NewRequest(req.Method, buf.String(), nil); err == nil {
		
		newReq.Header = req.Header
		newReq.URL.Path = req.URL.Path
		newReq.URL.Fragment = req.URL.Fragment

		// // Write the headers
		// for k, v := range newReq.Header {
		// 	newReq.Header[k] = v
		// }

		// Set the body to read from the incoming request - TODO: May need to kick off another goroutine to do this manually for slow connections, have some sort of pause if it can't read anything?
		newReq.Body = req.Body

		// Perform the request
		if resp, err := client.Do(newReq); err == nil {
			defer resp.Body.Close()

			if !(resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotModified) {
				return resp.StatusCode
			} else {
				w.WriteHeader(resp.StatusCode)

				// Copy response header into our response writer
				for k, v := range resp.Header {
					w.Header()[k] = v
				}

				// Write response body into ResponseWriter
				if resp.Body == nil || writeBody(w, resp) == io.EOF {
					return http.StatusOK
				} else {
					return http.StatusInternalServerError
				}
			}

		} else {
			Debug("+handleSocket - Error performing request:", err)
			return http.StatusInternalServerError
		}
	
	} else {
		Debug("+handleSocket - Error creating request")
		return http.StatusInternalServerError
	}
}

func writeBody(w http.ResponseWriter, resp *http.Response) error {
	reader := resp.Body

	// May have content but length is unknown...
	if resp.ContentLength <= 0 {

		b := make([]byte, 1)

		// Either EOF or an actual error here, we can just return either
		if _, err := reader.Read(b); err != nil {
			return err

		// Non empty body and we don't know size
		} else {
			return writeToResponse(w, make([]byte, BufferMax), &WrapperReader{ UnderlyingReader: reader, B: b[0], ByteRead: false} )
		}

	} else if resp.ContentLength > BufferMax {
		return writeToResponse(w, make([]byte, BufferMax), reader)
	
	} else {
		return writeToResponse(w, make([]byte, resp.ContentLength), reader)
	}
}

func writeToResponse(w http.ResponseWriter, buf []byte, resp io.ReadCloser) error {
	for {
		r, err := resp.Read(buf)
		if r == 0 {
			// Either reached end of file or we have an error
			if err != nil {
				return err 
			
			// Not received any data here but not err or EOF
			} else {
				// TODO - Throttle?
			}
		} else {
			w.Write(buf[:r])
		}
	}
}

type WrapperReader struct {
	UnderlyingReader io.ReadCloser
	B byte
	ByteRead bool
}

func (this *WrapperReader) Read(p []byte) (n int, err error) {
	if !this.ByteRead {
		p[0] = this.B
		this.ByteRead = true
		n, err = this.UnderlyingReader.Read(p[1:])
		n += 1
		return
	} else {
		n, err = this.UnderlyingReader.Read(p)
		return
	}
}

func (this *WrapperReader) Close() error {
	return this.UnderlyingReader.Close() 
}

