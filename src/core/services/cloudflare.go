package services

import (
	"YourPlace/src/core"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type DNSUpdate struct {
	Comment string   `json:"comment"`
	Content string   `json:"content"`
	Name    string   `json:"name"`
	Proxied bool     `json:"proxied"`
	Tags    []string `json:"tags"`
	Ttl     uint32   `json:"ttl"`
	Type    string   `json:"type"`
}

func CloudflareDDNSUpdate() {

	//CloudflareUpdateDNSRecord()
}

func CloudflareUpdateDNSRecord(email, key, domainName, zoneID, recordID string) { // https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/edit/
	publicIP := PublicIP()
	dnsRecord := DNSUpdate{
		Comment: "Domain verification record",
		Content: publicIP,
		Name:    domainName,
		Proxied: false,
		Tags:    []string{"owner:YourPlace"},
		Ttl:     60,
		Type:    "A",
	}
	jsonObj, err := json.Marshal(dnsRecord)
	if err != nil {
		core.LogError("Could not update Cloudflare DNS record: " + err.Error())
		return
	}
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", zoneID, recordID)
	request, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(jsonObj))
	if err != nil {
		core.LogError("Could not update Cloudflare DNS record: " + err.Error())
		return
	}
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("X-Auth-Email", email)
	request.Header.Set("X-Auth-Key", key)
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		core.LogError("Could not update Cloudflare DNS record: " + err.Error())
		return
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		core.LogError("Could not update Cloudflare DNS record: Non-200 response")
		return
	}
}
