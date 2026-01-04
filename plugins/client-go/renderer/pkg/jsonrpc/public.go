package jsonrpc

import (
	"context"
	"errors"
)

func (client *ClientRPC) Call(ctx context.Context, method string, params ...any) (response *ResponseRPC, err error) {

	request := &RequestRPC{
		ID:      NewID(),
		Method:  method,
		Params:  Params(params...),
		JSONRPC: Version,
	}
	return client.doCall(ctx, request)
}

func (client *ClientRPC) CallRaw(ctx context.Context, request *RequestRPC) (response *ResponseRPC, err error) {
	return client.doCall(ctx, request)
}

func (client *ClientRPC) CallFor(ctx context.Context, out any, method string, params ...any) (err error) {

	rpcResponse, err := client.Call(ctx, method, params...)
	if err != nil {
		return err
	}
	if rpcResponse.Error != nil {
		return rpcResponse.Error
	}
	return rpcResponse.GetObject(out)
}

func (client *ClientRPC) CallBatch(ctx context.Context, requests RequestsRPC) (responses ResponsesRPC, err error) {

	if len(requests) == 0 {
		err = errors.New("empty request list")
		return
	}
	return client.doBatchCall(ctx, requests)
}

func (client *ClientRPC) CallBatchRaw(ctx context.Context, requests RequestsRPC) (responses ResponsesRPC, err error) {

	if len(requests) == 0 {
		err = errors.New("empty request list")
		return
	}
	return client.doBatchCall(ctx, requests)
}
