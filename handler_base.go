package reverseproxy

type BaseHandler struct {

	// Resource is used to give the RequestHandler function some context on why it was called
	//
	// This is so it knows how to seek a file or what are the connection details for a socket. Whether to use compression
	// etc
	Resource *ServerResource

	// ErrorMap is used when an error occurs and we want to display an error page rather than send an error code
	ErrorMappings []ErrorMapping
}

