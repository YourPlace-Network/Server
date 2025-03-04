package services

import (
	"YourPlace/src/core"
	"YourPlace/src/core/security"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"runtime"
)

func GithubGetLatestName(projectPath string) (string, error) {
	url := security.SanitizeURL("https", "api.github.com", "/repos/"+projectPath+"/releases/latest")
	if !security.IsValidURL(url) {
		return "", core.LogErrorReturn("Invalid github URL: " + url)
	}
	resp, err := http.Get(url)
	if err != nil {
		return "", core.LogErrorReturn("Could not HTTP GET URL: " + url + " - " + err.Error())
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", core.LogErrorReturn("Could not read HTTP body for URL: " + url + " - " + err.Error())
	}
	var result map[string]interface{}
	json.Unmarshal([]byte(body), &result)
	name := result["name"].(string)
	return name, nil
}
func GetKuboLatestReleaseDownloadURL() (string, string, string, error) {
	projectPath := "ipfs/kubo"
	name, err := GithubGetLatestName(projectPath)
	if err != nil {
		return "", "", "", core.LogErrorReturn("Could not get github projects latest name: " + err.Error())
	}
	architecture := ""
	extension := ""
	binaryName := ""
	binaryHash := ""
	if runtime.GOOS == "windows" {
		extension = ".zip"
		architecture = "windows-amd64"
	} else if runtime.GOOS == "darwin" {
		extension = ".tar.gz"
		if runtime.GOARCH == "arm64" {
			architecture = "darwin-arm64"
		} else if runtime.GOARCH == "amd64" {
			architecture = "darwin-amd64"
		}
	} else if runtime.GOOS == "linux" {
		extension = ".tar.gz"
		if runtime.GOARCH == "arm" {
			architecture = "linux-arm64"
		} else if runtime.GOARCH == "amd64" {
			architecture = "linux-amd64"
		}
	}
	binaryName = "kubo_" + name + "_" + architecture + extension
	binaryHash = "kubo_" + name + "_" + architecture + extension + ".sha512"
	binaryUrl := security.SanitizeURL("https", "github.com", projectPath+"/releases/download/"+name+"/"+binaryName)
	hashUrl := security.SanitizeURL("https", "github.com", projectPath+"/releases/download/"+name+"/"+binaryHash)
	return binaryUrl, hashUrl, binaryName, nil
}
