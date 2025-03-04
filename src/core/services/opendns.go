package services

import (
	"YourPlace/src/core"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"time"
)

func OpenDNSGetPublicIP() (net.IP, error) {
	magicDomain := "https://myipv4.p1.opendns.com/get_my_ip"
	type response struct {
		IP string `json:"ip"`
	}
	client := &http.Client{
		Timeout: time.Second * 10,
	}
	maxRetries := 3
	var lastError error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Second * 2)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, magicDomain, nil)
		if err != nil {
			lastError = core.LogErrorReturn("failed to create OpenDNS request: " + err.Error())
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			lastError = core.LogErrorReturn("failed to send OpenDNS request: " + err.Error())
			continue
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		if resp.StatusCode != http.StatusOK {
			lastError = core.LogErrorReturn("Failed to resolve OpenDNS public IP: Non-OK status code")
			continue
		}
		var responseObj response
		err = json.Unmarshal(body, &responseObj)
		if err != nil {
			lastError = core.LogErrorReturn("Failed to parse OpenDNS response: " + err.Error())
			continue
		}
		ip := net.ParseIP(responseObj.IP)
		if ip == nil {
			lastError = core.LogErrorReturn("Failed to parse OpenDNS response, IP nil")
			continue
		}
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsMulticast() || ip.IsUnspecified() {
			lastError = core.LogErrorReturn("Invalid OpenDNS IP response")
			continue
		}
		return ip, nil
	}
	return nil, core.LogErrorReturn("Failed to resolve OpenDNS public IP: " + lastError.Error())
}

/*func OpenDNSGetPublicIP2() net.IP {
	magicDomain := "https://myip.dnsomatic.com/"
	resp, err := http.Get(magicDomain)
	if err != nil {
		core.LogError("Failed to resolve OpenDNS public IP: " + err.Error())
		return nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		core.LogError("Failed to read OpenDNS response: " + err.Error())
		return nil
	}
	ipString := strings.TrimSpace(string(body))
	ip := net.ParseIP(ipString)
	if ip == nil {
		core.LogError("Failed to parse OpenDNS response: " + ipString)
		return nil
	}
	return ip
}*/

/*func OpenDNSGetPublicIP() net.IP {
	magicDomain := "myip.opendns.com"
	resolverURL := "resolver1.opendns.com:53"
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialer := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000), // Set timeout to 10 seconds
			}
			return dialer.DialContext(ctx, "udp", resolverURL)
		},
	}
	ip, err := resolver.LookupHost(context.Background(), magicDomain)
	if err != nil {
		core.LogError("Failed to resolve OpenDNS public IP: " + err.Error())
		return nil
	}
	return net.ParseIP(ip[0])
}*/
