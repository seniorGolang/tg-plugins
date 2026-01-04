package jsonrpc

import (
	"net/http"
	"sync/atomic"
	"time"
)

const (
	Version = "2.0"
)

type ID = uint64

var (
	NilID     ID = 0
	requestID uint64
)

func NewID() ID {
	return atomic.AddUint64(&requestID, 1)
}

type ClientRPC struct {
	options    options
	endpoint   string
	httpClient *http.Client
}

func NewClient(endpoint string, opts ...Option) (client *ClientRPC) {

	defaultTransport := &http.Transport{
		MaxIdleConns:          300,
		MaxIdleConnsPerHost:   50,
		MaxConnsPerHost:       100,
		IdleConnTimeout:       60 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		DisableKeepAlives:     false,
		ForceAttemptHTTP2:     true,
	}
	defaultClient := &http.Client{
		Timeout:   10 * time.Second,
		Transport: defaultTransport,
	}
	client = &ClientRPC{
		endpoint:   endpoint,
		httpClient: defaultClient,
		options:    prepareOpts(opts),
	}
	if client.options.clientHTTP != nil {
		client.httpClient = client.options.clientHTTP
	}
	if client.options.tlsConfig != nil {
		if transport, ok := client.httpClient.Transport.(*http.Transport); ok {
			transport.TLSClientConfig = client.options.tlsConfig
		} else {
			existingTransport := client.httpClient.Transport
			newTransport := &http.Transport{
				MaxIdleConns:          300,
				MaxIdleConnsPerHost:   50,
				MaxConnsPerHost:       100,
				IdleConnTimeout:       60 * time.Second,
				ResponseHeaderTimeout: 10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				DisableKeepAlives:     false,
				ForceAttemptHTTP2:     true,
				TLSClientConfig:       client.options.tlsConfig,
			}
			if existingTransport != nil {
				if existingHTTPTransport, ok := existingTransport.(*http.Transport); ok {
					newTransport.MaxIdleConns = existingHTTPTransport.MaxIdleConns
					newTransport.MaxIdleConnsPerHost = existingHTTPTransport.MaxIdleConnsPerHost
					newTransport.MaxConnsPerHost = existingHTTPTransport.MaxConnsPerHost
					newTransport.IdleConnTimeout = existingHTTPTransport.IdleConnTimeout
					newTransport.ResponseHeaderTimeout = existingHTTPTransport.ResponseHeaderTimeout
					newTransport.ExpectContinueTimeout = existingHTTPTransport.ExpectContinueTimeout
					newTransport.TLSHandshakeTimeout = existingHTTPTransport.TLSHandshakeTimeout
					newTransport.DisableKeepAlives = existingHTTPTransport.DisableKeepAlives
					newTransport.ForceAttemptHTTP2 = existingHTTPTransport.ForceAttemptHTTP2
				}
			}
			client.httpClient.Transport = newTransport
		}
	}
	return client
}
