package network

import (
	"YourPlace/src/core"
	"YourPlace/src/core/security"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

func HttpGet(url string, timeout uint64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	if !security.IsValidURL(url) {
		return "", core.LogErrorReturn("Invalid URL")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", core.LogErrorReturn("Could not create request")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", core.LogErrorReturn("Could not send request")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", core.LogErrorReturn("Got non-200 response")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", core.LogErrorReturn("Could not ready response body")
	}
	return string(body), nil
}
func HttpGetFile(url string, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("got non-200 response")
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}
	return nil
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
func HttpResolveRedirect(targetUrl string) string {
	if !security.IsValidURL(targetUrl) {
		return targetUrl
	}
	client := &http.Client{
		Timeout: time.Second * 10,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(http.MethodHead, targetUrl, nil)
	if err != nil {
		return targetUrl
	}
	resp, err := client.Do(req)
	if err != nil {
		return targetUrl
	}
	defer resp.Body.Close()
	location := resp.Header.Get("Location")
	if location == "" || !security.IsValidURL(location) {
		return targetUrl
	}
	return location
}
func HttpPost(url string) (string, error) {
	if !security.IsValidURL(url) {
		return "", core.LogErrorReturn("Invalid URL")
	}
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return "", core.LogWarningReturn("Could not create HTTP request: " + err.Error())
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", core.LogWarningReturn("Could not send HTTP request: " + err.Error())
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", core.LogWarningReturn("Could not read response body: " + err.Error())
	}
	if resp.StatusCode != http.StatusOK {
		return string(body), core.LogWarningReturn("Got non-200 response: " + strconv.Itoa(resp.StatusCode) + "\n" + string(body))
	}
	return string(body), nil
}
