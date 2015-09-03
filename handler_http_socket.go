package reverseproxy

import (
	"net/http"
	"io"
	"github.com/seanjohnno/objpool"
)

var (
	client = &http.Client{ }
)

const (
	BufferExpiryTime = 3000 // 3 seconds
	BufferMax = 1024
)

type HttpHandler struct {

	// FSHandler contains ServerResource & ErrorMappings map
	FSHandler

	BufferPool objpool.ObjectPool
}

// NewHttpHandler returns an *NewHttpHandler
func NewHttpHandler(rsc *ServerResource, errorMappings []ErrorMapping) (*HttpHandler) {
	
	// FileAccessor handles null cache
	return &HttpHandler{ FSHandler: *NewFSHandler( rsc, errorMappings, nil ), BufferPool: objpool.NewTimedExiryPool(BufferExpiryTime) }
}

func (this *HttpHandler) HandleRequest(w http.ResponseWriter, req *http.Request) {
	Debug("+HandlerHttpSocket - Loading from http connection")
	useCompression := this.shouldUseCompression(req)
	if status := this.HandleSocket(w, req); !(status == http.StatusOK || status == http.StatusNotModified) {
		this.handleError(w, req, status, useCompression)
	}
}

func (this * HttpHandler) HandleSocket(w http.ResponseWriter, req *http.Request) int {

	Debug("+handleSocket - Method:", req.Method, "URL:", this.Resource.Path)

	// Create the request
	if newReq, err := http.NewRequest(req.Method, this.Resource.Path, nil); err == nil {
		
		newReq.Header = req.Header
		newReq.URL.Path = req.URL.Path
		newReq.URL.Fragment = req.URL.Fragment

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
				if resp.Body == nil || this.writeBody(w, resp) == io.EOF {
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

func (this * HttpHandler) writeBody(w http.ResponseWriter, resp *http.Response) error {
	reader := resp.Body

	// May have content but length is unknown...
	if resp.ContentLength <= 0 {

		b := make([]byte, 1)

		// Either EOF or an actual error here, we can just return either
		if _, err := reader.Read(b); err != nil {
			return err

		// Non empty body and we don't know size
		} else {
			return this.writeToResponse(w, this.getByteBuffer(), &WrapperReader{ UnderlyingReader: reader, B: b[0], ByteRead: false} )
		}

	// TODO - Is it better to allocate ContentLength here or keep buffer size the same so they can all be fetched from the common pool
	} else {
		return this.writeToResponse(w, this.getByteBuffer(), reader)
	}
}

func (this *HttpHandler) writeToResponse(w http.ResponseWriter, buf []byte, resp io.ReadCloser) error {
	for {
		r, err := resp.Read(buf)
		if r == 0 {
			// Either reached end of file or we have an error
			if err != nil {
				this.BufferPool.Add(buf)
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

func (this *HttpHandler) getByteBuffer() ([]byte) {
	if buf, present := this.BufferPool.Retrieve(); present {
		return buf.([]byte)
	} else {
		return make([]byte, BufferMax)
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

