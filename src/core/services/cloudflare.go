package services

import (
	"YourPlace/src/core"
	"bytes"
	"encoding/json"
	"net/http"
)

type DNSUpdate struct {
	comment string
	content string
	name    string
	proxied bool
	tags    []string
	ttl     int32
}

func updateDNSRecord(email string, key string, domainName string) {
	obj := DNSUpdate{comment: "Domain verification record", content: PublicIP(), name: domainName, tags: []string{"owner:YourPlace"}, ttl: 3600}
	jsonObj, err := json.Marshal(obj)
	if err != nil {
		core.LogError("Could not update Cloudflare DNS record: " + err.Error())
		return
	}
	request, err := http.NewRequest(http.MethodPut,
		"https://api.cloudflare.com/client/v4/zones/zone_identifier/dns_records/identifier",
		bytes.NewBuffer(jsonObj))
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

/*

func GetLatestRelease(projectPath string) (bool, string) {
	url := security.SanitizeURL("https", "github.com", projectPath+"/release/latest")
	if !security.IsValidURL(url) {
		return false, ""
	}
	resp, err := http.Get(url)
	if err != nil {
		print(err)
		return false, ""
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		print(err)
		return false, ""
	}
	fmt.Print(string(body))
	//todo
	return false, ""
}

*/
