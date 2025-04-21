package domain

type ParsedRequest struct {
	Method     string                 `msgpack:"method"`
	Path       string                 `msgpack:"path"`
	GetParams  map[string]interface{} `msgpack:"get_params"`
	Headers    map[string]string      `msgpack:"headers"`
	Cookies    map[string]string      `msgpack:"cookies"`
	PostParams map[string]interface{} `msgpack:"post_params"`
	Body       string                 `msgpack:"body"`
	Host       string                 `msgpack:"host"`
	RawRequest string                 `msgpack:"raw_request"`
}

type ParsedResponse struct {
	Code    int               `msgpack:"code"`
	Message string            `msgpack:"message"`
	Headers map[string]string `msgpack:"headers"`
	Body    string            `msgpack:"body"`
}
