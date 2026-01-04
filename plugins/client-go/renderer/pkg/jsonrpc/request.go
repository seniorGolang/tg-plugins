package jsonrpc

type RequestRPC struct {
	ID      ID     `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
	JSONRPC string `json:"jsonrpc"`
}

type RequestsRPC []*RequestRPC

func NewRequest(method string, params ...any) *RequestRPC {

	request := &RequestRPC{
		ID:      NewID(),
		Method:  method,
		Params:  Params(params...),
		JSONRPC: Version,
	}
	return request
}

func NewRequestWithID(id ID, method string, params ...any) *RequestRPC {

	request := &RequestRPC{
		ID:      id,
		Method:  method,
		Params:  Params(params...),
		JSONRPC: Version,
	}
	return request
}
