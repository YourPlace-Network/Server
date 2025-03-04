package services

import (
	"YourPlace/src/core"
	"net/http"
	"net/url"
	"time"
)

type DNSResponse struct {
	Answer []struct {
		Data string `json:"data"`
	} `json:"Answer"`
}

func SendGoogleAnalyticsEvent(category string, action string, label string) {
	data := url.Values{
		"v":   {"1"},
		"tid": {"G-TCHR7RGE46"}, // Replace with your Tracking ID
		"cid": {"555"},          // Client ID
		"t":   {"event"},        // Event hit type
		"ec":  {category},       // Event Category
		"ea":  {action},         // Event Action
		"el":  {label},          // Event label
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequest("POST", "https://www.google-analytics.com/mp/collect", nil)
	if err != nil {
		core.LogWarn("Failed to send analytics event: " + err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.URL.RawQuery = data.Encode()
	resp, err := client.Do(req)
	if err != nil {
		core.LogWarn("Failed to send analytics event: " + err.Error())
		return
	}
	defer resp.Body.Close()
}

/*
	func GoogleGetPublicIP() net.IP {
		resolverURL := "ns1.google.com:53"
		magicDomain := "o-o.myaddr.l.google.com"
		resolver := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				dialer := net.Dialer{
					Timeout: time.Millisecond * time.Duration(10000), // Set timeout to 10 seconds
				}
				return dialer.DialContext(ctx, "udp", resolverURL)
			},
		}
		txtRecords, err := resolver.LookupTXT(context.Background(), magicDomain)
		if err != nil {
			core.LogWarn("Failed to resolve public IP from Google: " + err.Error())
			return nil
		}
		for _, txt := range txtRecords {
			ip := net.ParseIP(txt)
			if ip != nil {
				return ip
			}
		}
		core.LogWarn("Failed to resolve public IP from Google")
		return nil
	}
*/
