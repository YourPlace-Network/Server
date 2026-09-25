//go:build darwin

package launcher

import (
	"YourPlace/src/core/host"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

func OpenLauncherUI(protocol, domain string, port int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client := &http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	localURL := protocol + "://localhost:" + strconv.Itoa(port)
	openDomain := domain
	if origin := os.Getenv("YOURPLACE_ORIGIN"); origin != "" {
		openDomain = origin
	}
	openURL := protocol + "://" + openDomain + ":" + strconv.Itoa(port)
	if launcherServerReady(ctx, client, localURL) {
		host.OpenBrowser(openURL)
		return nil
	}
	if err := startLauncherServer(ctx); err != nil {
		return err
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		if launcherServerReady(ctx, client, localURL) {
			host.OpenBrowser(openURL)
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for YourPlace")
		case <-ticker.C:
		}
	}
}
func launcherServerReady(ctx context.Context, client *http.Client, localURL string) bool {
	var result struct {
		Status string `json:"status"`
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, localURL+"/ping", nil)
	if err != nil {
		return false
	}
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusTemporaryRedirect && response.Header.Get("Location") == "/setup" {
		return true
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&result); err != nil {
		return false
	}
	return (response.StatusCode == http.StatusOK && result.Status == "pong") ||
		(response.StatusCode == http.StatusServiceUnavailable && result.Status == "Not installed")
}
func startLauncherServer(ctx context.Context) error {
	userDomain := "gui/" + strconv.Itoa(os.Getuid())
	service := userDomain + "/com.yourplace.server"
	if err := exec.CommandContext(ctx, "/bin/launchctl", "print", service).Run(); err != nil {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		plist := filepath.Join(home, "Library", "LaunchAgents", "com.yourplace.server.plist")
		if err := exec.CommandContext(ctx, "/bin/launchctl", "bootstrap", userDomain, plist).Run(); err != nil {
			if checkErr := exec.CommandContext(ctx, "/bin/launchctl", "print", service).Run(); checkErr != nil {
				return fmt.Errorf("could not load YourPlace LaunchAgent: %w", err)
			}
		}
	}
	if err := exec.CommandContext(ctx, "/bin/launchctl", "kickstart", service).Run(); err != nil {
		return fmt.Errorf("could not start YourPlace: %w", err)
	}
	return nil
}
