package blockchain

import (
	"YourPlace/src/core"
	"bytes"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type BaseRPCProxy struct {
	targetURL    string
	rateLimit    int
	requestQueue chan *proxyRequest
	mu           sync.RWMutex
	client       *http.Client
}

type proxyRequest struct {
	body     []byte
	response chan *proxyResponse
}

type proxyResponse struct {
	statusCode int
	body       []byte
	headers    http.Header
	err        error
}

var (
	baseRPCProxy     *BaseRPCProxy
	baseRPCProxyOnce sync.Once
)

func InitBaseRPCProxy(targetURL string, rateLimit int) *BaseRPCProxy {
	baseRPCProxyOnce.Do(func() {
		baseRPCProxy = &BaseRPCProxy{
			targetURL:    targetURL,
			rateLimit:    rateLimit,
			requestQueue: make(chan *proxyRequest, 1000),
			client: &http.Client{
				Timeout: 30 * time.Second,
			},
		}
		go baseRPCProxy.processQueue()
		core.LogInfo("Base RPC Proxy initialized with rate limit: " + strconv.Itoa(rateLimit) + " requests/second")
	})
	return baseRPCProxy
}

func GetBaseRPCProxy() *BaseRPCProxy {
	return baseRPCProxy
}

func (p *BaseRPCProxy) UpdateTargetURL(url string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.targetURL = url
	core.LogInfo("Base RPC Proxy target URL updated: " + url)
}

func (p *BaseRPCProxy) UpdateRateLimit(rateLimit int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rateLimit = rateLimit
	core.LogInfo("Base RPC Proxy rate limit updated: " + strconv.Itoa(rateLimit))
}

func (p *BaseRPCProxy) processQueue() {
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

func (p *BaseRPCProxy) forwardRequest(body []byte) *proxyResponse {
	p.mu.RLock()
	targetURL := p.targetURL
	p.mu.RUnlock()
	if targetURL == "" {
		return &proxyResponse{
			statusCode: http.StatusServiceUnavailable,
			body:       []byte(`{"error": "RPC proxy not configured"}`),
			err:        nil,
		}
	}
	req, err := http.NewRequest("POST", targetURL, bytes.NewReader(body))
	if err != nil {
		return &proxyResponse{
			statusCode: http.StatusInternalServerError,
			body:       []byte(`{"error": "Failed to create request"}`),
			err:        err,
		}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return &proxyResponse{
			statusCode: http.StatusBadGateway,
			body:       []byte(`{"error": "Failed to reach RPC endpoint"}`),
			err:        err,
		}
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &proxyResponse{
			statusCode: http.StatusInternalServerError,
			body:       []byte(`{"error": "Failed to read response"}`),
			err:        err,
		}
	}
	return &proxyResponse{
		statusCode: resp.StatusCode,
		body:       respBody,
		headers:    resp.Header,
		err:        nil,
	}
}

func (p *BaseRPCProxy) ProxyRequest(body []byte) (int, []byte, error) {
	req := &proxyRequest{
		body:     body,
		response: make(chan *proxyResponse, 1),
	}
	select {
	case p.requestQueue <- req:
		resp := <-req.response
		return resp.statusCode, resp.body, resp.err
	case <-time.After(60 * time.Second):
		return http.StatusServiceUnavailable, []byte(`{"error": "Request queue timeout"}`), nil
	}
}

func (p *BaseRPCProxy) HandleHTTP(w http.ResponseWriter, r *http.Request) {
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
		core.LogError("RPC proxy error: " + err.Error())
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(respBody)
}
