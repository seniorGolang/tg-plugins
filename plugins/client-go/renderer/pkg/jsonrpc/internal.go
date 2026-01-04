package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

func (client *ClientRPC) newRequest(ctx context.Context, reqBody any) (request *http.Request, err error) {

	bodyReader := bytes.NewBuffer(nil)
	if err = json.NewEncoder(bodyReader).Encode(reqBody); err != nil {
		return
	}
	if request, err = http.NewRequestWithContext(ctx, http.MethodPost, client.endpoint, bodyReader); err != nil {
		return
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	for k, v := range client.options.customHeaders {
		if k == "Host" {
			request.Host = v
		} else {
			request.Header.Set(k, v)
		}
	}
	for _, header := range client.options.headersFromCtx {
		if value := ctx.Value(header); value != nil {
			if k := toString(header); k != "" {
				if v := toString(value); v != "" {
					request.Header.Set(k, v)
				}
			}
		}
	}
	return
}

func (client *ClientRPC) doCall(ctx context.Context, request *RequestRPC) (rpcResponse *ResponseRPC, err error) {

	var httpRequest *http.Request
	if httpRequest, err = client.newRequest(ctx, request); err != nil {
		err = fmt.Errorf("rpc call %v() on %v: %v", request.Method, client.endpoint, err.Error())
		return
	}
	if client.options.before != nil {
		ctx = client.options.before(ctx, httpRequest)
	}
	if client.options.logRequests {
		if cmd, cmdErr := ToCurl(httpRequest); cmdErr == nil {
			slog.DebugContext(ctx, "call", slog.String("method", request.Method), slog.String("curl", cmd.String()))
		}
	}
	defer func() {
		if err != nil && client.options.logOnError {
			if cmd, cmdErr := ToCurl(httpRequest); cmdErr == nil {
				slog.ErrorContext(ctx, "call", slog.String("method", request.Method), slog.String("curl", cmd.String()), slog.Any("error", err))
			}
		}
	}()
	var httpResponse *http.Response
	if httpResponse, err = client.httpClient.Do(httpRequest); err != nil {
		err = fmt.Errorf("rpc call %v() on %v: %v", request.Method, httpRequest.URL.String(), err.Error())
		return
	}
	defer httpResponse.Body.Close()
	if client.options.after != nil {
		if err = client.options.after(ctx, httpResponse); err != nil {
			return
		}
	}
	if httpResponse.StatusCode != http.StatusOK {
		bodyBytes, readErr := io.ReadAll(io.LimitReader(httpResponse.Body, 1024))
		errorMsg := string(bodyBytes)
		if readErr != nil || errorMsg == "" {
			errorMsg = httpResponse.Status
		}
		return nil, &HTTPError{
			Code: httpResponse.StatusCode,
			err:  fmt.Errorf("rpc call %v() on %v status code: %v. %v", request.Method, httpRequest.URL.String(), httpResponse.StatusCode, errorMsg),
		}
	}
	decoder := json.NewDecoder(httpResponse.Body)
	if !client.options.allowUnknownFields {
		decoder.DisallowUnknownFields()
	}
	decoder.UseNumber()
	err = decoder.Decode(&rpcResponse)
	if err != nil {
		return nil, fmt.Errorf("rpc call %v() on %v status code: %v. could not decode body to rpc response: %v", request.Method, httpRequest.URL.String(), httpResponse.StatusCode, err.Error())
	}
	if rpcResponse == nil {
		err = fmt.Errorf("rpc call %v() on %v status code: %v. rpc response missing", request.Method, httpRequest.URL.String(), httpResponse.StatusCode)
		return
	}
	if rpcResponse.ID != request.ID {
		return nil, fmt.Errorf("rpc call %v() on %v: response ID mismatch. expected: %v, got: %v", request.Method, httpRequest.URL.String(), request.ID, rpcResponse.ID)
	}
	return
}

func (client *ClientRPC) doBatchCall(ctx context.Context, rpcRequests []*RequestRPC) (rpcResponses ResponsesRPC, err error) {

	defer func() {
		if err != nil {
			for _, request := range rpcRequests {
				if request.ID == NilID {
					continue
				}
				rpcResponses = append(rpcResponses, &ResponseRPC{
					ID:      request.ID,
					JSONRPC: request.JSONRPC,
					Error: &RPCError{
						Message: err.Error(),
					},
				})
			}
		}
	}()
	var httpRequest *http.Request
	if httpRequest, err = client.newRequest(ctx, rpcRequests); err != nil {
		err = fmt.Errorf("rpc batch call on %v: %v", client.endpoint, err.Error())
		return
	}
	if client.options.before != nil {
		ctx = client.options.before(ctx, httpRequest)
	}
	if client.options.logRequests {
		if cmd, cmdErr := ToCurl(httpRequest); cmdErr == nil {
			slog.DebugContext(ctx, "call", slog.String("method", "batch"), slog.Int("count", len(rpcRequests)), slog.String("curl", cmd.String()))
		}
	}
	defer func() {
		if err != nil && client.options.logOnError {
			if cmd, cmdErr := ToCurl(httpRequest); cmdErr == nil {
				slog.ErrorContext(ctx, "call", slog.String("method", "batch"), slog.Int("count", len(rpcRequests)), slog.String("curl", cmd.String()), slog.Any("error", err))
			}
		}
	}()
	var httpResponse *http.Response
	if httpResponse, err = client.httpClient.Do(httpRequest); err != nil {
		err = fmt.Errorf("rpc batch call on %v: %v", httpRequest.URL.String(), err.Error())
		return
	}
	defer httpResponse.Body.Close()
	if client.options.after != nil {
		if err = client.options.after(ctx, httpResponse); err != nil {
			return
		}
	}
	if httpResponse.StatusCode != http.StatusOK {
		bodyBytes, readErr := io.ReadAll(httpResponse.Body)
		errorMsg := string(bodyBytes)
		if readErr != nil || errorMsg == "" {
			errorMsg = httpResponse.Status
		}
		return nil, &HTTPError{
			Code: httpResponse.StatusCode,
			err:  fmt.Errorf("rpc batch call on %v status code: %v. %v", httpRequest.URL.String(), httpResponse.StatusCode, errorMsg),
		}
	}
	decoder := json.NewDecoder(httpResponse.Body)
	if !client.options.allowUnknownFields {
		decoder.DisallowUnknownFields()
	}
	decoder.UseNumber()
	err = decoder.Decode(&rpcResponses)
	if err != nil {
		err = fmt.Errorf("rpc batch call on %v status code: %v. could not decode body to rpc response: %v", httpRequest.URL.String(), httpResponse.StatusCode, err.Error())
		return
	}
	if len(rpcResponses) == 0 {
		err = fmt.Errorf("rpc batch call on %v status code: %v. rpc response missing", httpRequest.URL.String(), httpResponse.StatusCode)
		return
	}
	requestIDMap := make(map[ID]bool, len(rpcRequests))
	for _, req := range rpcRequests {
		if req.ID != NilID {
			requestIDMap[req.ID] = true
		}
	}
	for _, resp := range rpcResponses {
		if resp.ID != NilID && !requestIDMap[resp.ID] {
			err = fmt.Errorf("rpc batch call on %v: response ID %v does not match any request ID", httpRequest.URL.String(), resp.ID)
			return
		}
	}
	return
}
