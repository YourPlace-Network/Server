package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type PackageJSON struct {
	Dependencies    map[string]string `json:"dependencies,omitempty"`
	DevDependencies map[string]string `json:"devDependencies,omitempty"`
}
type NPMResponse struct {
	DistTags struct {
		Latest string `json:"latest"`
	} `json:"dist-tags"`
}

func main() {
	path := "package.json" // Change this to your desired path

	err := updatePackageJSON(path)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Successfully updated package.json")
}

func updatePackageJSON(path string) error {
	// Read package.json
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read package.json: %w", err)
	}
	// Parse JSON while preserving original structure
	var rawJSON map[string]interface{}
	err = json.Unmarshal(data, &rawJSON)
	if err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}
	var pkg PackageJSON
	err = json.Unmarshal(data, &pkg)
	if err != nil {
		return fmt.Errorf("failed to parse package structure: %w", err)
	}
	// Update dependencies
	if pkg.Dependencies != nil {
		for depName := range pkg.Dependencies {
			latestVersion, err := getLatestVersion(depName)
			if err != nil {
				fmt.Printf("Warning: failed to get latest version for %s: %v\n", depName, err)
				continue
			}
			pkg.Dependencies[depName] = "^" + latestVersion
		}
		rawJSON["dependencies"] = pkg.Dependencies
	}
	// Update devDependencies
	if pkg.DevDependencies != nil {
		for depName := range pkg.DevDependencies {
			latestVersion, err := getLatestVersion(depName)
			if err != nil {
				fmt.Printf("Warning: failed to get latest version for %s: %v\n", depName, err)
				continue
			}
			pkg.DevDependencies[depName] = "^" + latestVersion
		}
		rawJSON["devDependencies"] = pkg.DevDependencies
	}
	// Write back to file with proper formatting
	updatedData, err := json.MarshalIndent(rawJSON, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	err = os.WriteFile(path, updatedData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write package.json: %w", err)
	}
	return nil
}
func getLatestVersion(packageName string) (string, error) {
	url := fmt.Sprintf("https://registry.npmjs.org/%s", packageName)
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("npm registry returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	var npmResp NPMResponse
	err = json.Unmarshal(body, &npmResp)
	if err != nil {
		return "", fmt.Errorf("failed to parse npm response: %w", err)
	}
	if npmResp.DistTags.Latest == "" {
		return "", fmt.Errorf("no latest version found")
	}
	return npmResp.DistTags.Latest, nil
}
