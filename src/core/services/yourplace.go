package services

import (
	"YourPlace/src/core"
	"YourPlace/src/core/host"
	"YourPlace/src/core/security"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	homeURL  = "https://yourplace.network"
	YpPrefix = "yp/1"
)

func PublicIP() string {
	type PublicIP struct {
		IPaddress string `json:"ipaddress"`
	}
	var data PublicIP
	response, err := http.Get(homeURL + "/meta/ipaddress")
	if err != nil {
		core.LogError("Could not get public IP: " + err.Error())
		return ""
	}
	defer response.Body.Close()
	err = json.NewDecoder(response.Body).Decode(&data)
	if err != nil {
		core.LogError("Could not get public IP: " + err.Error())
		return ""
	}
	return data.IPaddress
}
func Analytics(version string) bool {
	req, _ := http.NewRequest("POST", homeURL+"/meta/analytics", nil)
	form := url.Values{}
	form.Add("version", host.GetServerVersion())
	form.Add("os", host.GetOS())
	form.Add("arch", host.GetCPUVendor())
	form.Add("bit", strconv.Itoa(int(host.GetCPUArch())))
	form.Add("keyboard", host.GetKeyboardLayout())
	req.PostForm = form
	req.Header.Add("Content-Type", "application/json")
	response, err := http.PostForm(homeURL+"/meta/analytics", form)
	if err != nil {
		core.LogError("YourPlace Analytics could not send payload: " + err.Error())
		return false
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		core.LogError("YourPlace Analytics call returned: " + response.Status)
		return false
	}
	return true
}
func GetLatestServerVersion() string {
	type Payload struct {
		Version string `json:"version" required:"true"`
	}
	var payload Payload
	err := HttpGetJson(homeURL+"/version", &payload)
	if err != nil {
		core.LogError("Could not get latest server version")
		return ""
	}
	if !security.IsValidYourPlaceVersion(payload.Version) {
		core.LogError("Invalid server version")
		return ""
	}
	return payload.Version
}

func HttpGetJson(url string, item interface{}) error {
	// https://stackoverflow.com/questions/17156371/how-to-get-json-response-from-http-get#31129967
	client := &http.Client{
		Timeout: time.Second * 30,
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return errors.New("got non-200 response")
	}
	err = json.Unmarshal(body, item)
	if err != nil {
		return err
	}
	return nil
}
