package tests

import (
	"html"
	"io"
	"net/http"
	"regexp"
	"testing"
)

func TestDatabaseSnapshotExport(t *testing.T) {
	//t.Skip() // todo debug
	csrfToken := GetCSRFToken("http://localhost:42424/settings/")
	if csrfToken == "" {
		t.Error("Failed to get CSRF token")
		return
	}
	t.Log(csrfToken)

	client := &http.Client{}
	req, _ := http.NewRequest("POST", "http://localhost:42424/settings/database/exportSnapshot", nil)
	req.Header.Set("X-CSRF-Token", csrfToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Error(err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Error("Failed to export database snapshot: " + resp.Status)
		return
	}
}
func TestDatabaseSnapshotImport(t *testing.T) {
	t.Skip() // todo debug

	//network.HttpPostJson()
}

func GetCSRFToken(URL string) string {
	client := &http.Client{}
	req, err := http.NewRequest("GET", URL, nil)
	if err != nil {
		return ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	bodyStr := string(body)
	csrfTokenRegex := regexp.MustCompile(`<input type="hidden" id="csrfToken" value="([^"]+)">`)
	matches := csrfTokenRegex.FindStringSubmatch(bodyStr)
	if len(matches) < 2 {
		return ""
	}
	csrfToken := matches[1]
	csrfToken = html.UnescapeString(csrfToken)
	return csrfToken
}
