package network

import (
	"YourPlace/src/core"
	"YourPlace/src/core/security"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
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

	/*host.CreateFolder(path)
	filename := filepath.Base(url)
	_filepath := filepath.Join(path, filename)
	if host.DoesExist(_filepath) {
		return core.LogWarningReturn("Downloading file already exists: " + _filepath)
	}
	out, err := os.Create(_filepath)
	if err != nil {
		return core.LogErrorReturn("Could not create file: " + _filepath)
	}
	defer out.Close()
	resp, err := http.Get(url)
	if err != nil {
		return core.LogErrorReturn("Could not download file: " + url)
	}
	defer resp.Body.Close()
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return core.LogErrorReturn("Could not write to file: " + _filepath)
	}*/
	return nil
} // todo
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
func HttpPostJson(url string, buffer bytes.Buffer) (string, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("POST", url, &buffer)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", core.LogErrorReturn("Could not send HTTP request trying to POST JSON")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", core.LogErrorReturn("Got non-200 response trying to POST JSON")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", core.LogErrorReturn("Could not read response body trying to POST JSON")
	}
	return string(body), nil
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
		return "", core.LogWarningReturn("Got non-200 response: " + strconv.Itoa(resp.StatusCode) + "\n" + string(body))
	}
	return string(body), nil
}
func HttpDownloadFile(url string, path string) error {
	/*host.CreateFolder(path)
	filename := filepath.Base(url)
	_filepath := filepath.Join(path, filename)
	if host.DoesExist(_filepath) {
		return core.LogWarningReturn("Downloading file already exists: " + _filepath)
	}
	out, err := os.Create(_filepath)
	if err != nil {
		return core.LogErrorReturn("Could not create file: " + _filepath)
	}
	defer out.Close()
	response, err := http.Get(url)
	if err != nil {
		return core.LogErrorReturn("Could not download file: " + url)
	}
	defer response.Body.Close()
	_, err = io.Copy(out, response.Body)
	if err != nil {
		return core.LogErrorReturn("Could not write to file: " + _filepath)
	}*/
	return nil
} // todo
