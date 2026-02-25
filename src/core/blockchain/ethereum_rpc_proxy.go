package blockchain

import (
	"YourPlace/src/core"
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type EthereumRPCProxy struct {
	client       *http.Client
	mu           sync.RWMutex
	rateLimit    int
	requestQueue chan *ethereumProxyRequest
	targetURL    string
}
type ethereumProxyRequest struct {
	body     []byte
	response chan *ethereumProxyResponse
}
type ethereumProxyResponse struct {
	body       []byte
	err        error
	headers    http.Header
	statusCode int
}

var (
	ethereumRPCProxy     *EthereumRPCProxy
	ethereumRPCProxyOnce sync.Once
)

const DefaultPublicEthereumRPC = "https://cloudflare-eth.com"
const DefaultPublicEthereumRPCRateLimit = 5

func InitEthereumRPCProxy(targetURL string, rateLimit int) *EthereumRPCProxy {
	ethereumRPCProxyOnce.Do(func() {
		if targetURL == "" || strings.HasPrefix(targetURL, "/") {
			targetURL = DefaultPublicEthereumRPC
			rateLimit = DefaultPublicEthereumRPCRateLimit
			core.LogDebug("Ethereum RPC Proxy using public fallback: " + DefaultPublicEthereumRPC)
		}
		ethereumRPCProxy = &EthereumRPCProxy{
			targetURL:    targetURL,
			rateLimit:    rateLimit,
			requestQueue: make(chan *ethereumProxyRequest, 1000),
			client: &http.Client{
				Timeout: 30 * time.Second,
			},
		}
		go ethereumRPCProxy.processQueue()
		core.LogDebug("Ethereum RPC Proxy initialized with rate limit: " + strconv.Itoa(rateLimit) + " requests/second")
	})
	return ethereumRPCProxy
}

func (p *EthereumRPCProxy) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	statusCode, respBody, err := p.ProxyRequest(body)
	if err != nil {
		core.LogDebug("Ethereum RPC proxy error: " + err.Error())
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(respBody)
}
func (p *EthereumRPCProxy) ProxyRequest(body []byte) (int, []byte, error) {
	req := &ethereumProxyRequest{
		body:     body,
		response: make(chan *ethereumProxyResponse, 1),
	}
	select {
	case p.requestQueue <- req:
		resp := <-req.response
		return resp.statusCode, resp.body, resp.err
	case <-time.After(60 * time.Second):
		return http.StatusServiceUnavailable, []byte(`{"error": "Request queue timeout"}`), nil
	}
}
func (p *EthereumRPCProxy) UpdateRateLimit(rateLimit int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rateLimit = rateLimit
	core.LogDebug("Ethereum RPC Proxy rate limit updated: " + strconv.Itoa(rateLimit))
}
func (p *EthereumRPCProxy) UpdateTargetURL(url string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.targetURL = url
}
func (p *EthereumRPCProxy) forwardRequest(body []byte) *ethereumProxyResponse {
	p.mu.RLock()
	targetURL := p.targetURL
	p.mu.RUnlock()
	if targetURL == "" {
		core.LogDebug("Ethereum RPC proxy: targetURL is empty")
		return &ethereumProxyResponse{
			statusCode: http.StatusServiceUnavailable,
			body:       []byte(`{"error": "RPC proxy not configured"}`),
			err:        nil,
		}
	}
	req, err := http.NewRequest("POST", targetURL, bytes.NewReader(body))
	if err != nil {
		core.LogDebug("Ethereum RPC proxy: failed to create request: " + err.Error())
		return &ethereumProxyResponse{
			statusCode: http.StatusInternalServerError,
			body:       []byte(`{"error": "Failed to create request"}`),
			err:        err,
		}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		core.LogDebug("Ethereum RPC proxy: failed to reach endpoint: " + err.Error())
		return &ethereumProxyResponse{
			statusCode: http.StatusBadGateway,
			body:       []byte(`{"error": "Failed to reach RPC endpoint"}`),
			err:        err,
		}
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		core.LogDebug("Ethereum RPC proxy: failed to read response: " + err.Error())
		return &ethereumProxyResponse{
			statusCode: http.StatusInternalServerError,
			body:       []byte(`{"error": "Failed to read response"}`),
			err:        err,
		}
	}
	return &ethereumProxyResponse{
		statusCode: resp.StatusCode,
		body:       respBody,
		headers:    resp.Header,
		err:        nil,
	}
}
func (p *EthereumRPCProxy) processQueue() {
	for req := range p.requestQueue {
		p.mu.RLock()
		rateLimit := p.rateLimit
		p.mu.RUnlock()
		if rateLimit > 0 {
			time.Sleep(time.Second / time.Duration(rateLimit))
		}
		resp := p.forwardRequest(req.body)
		req.response <- resp
	}
}
