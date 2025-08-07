package network

import (
	"YourPlace/src/core"
	"YourPlace/src/core/host"
	"golang.org/x/net/html"
	"regexp"
	"strings"
)

func StartTorHiddenService() (string, error) {
	host.CreateFolder(host.GetInstallDir() + "tor/hidden_service")
	host.RunShellCommandNoWait(host.GetInstallDir() + "tor/tor/YourPlaceTor --RunAsDaemon 1 --HiddenServiceDir " + host.GetInstallDir() + "tor/hidden_service --HiddenServicePort 42424")
	return "", nil
}
func StopTorHiddenService() (string, error) {
	ret := host.KillProcess("YourPlaceTor")
	if ret != true {
		return "", core.LogErrorReturn("Error stopping YourPlaceTor")
	}
	return "", nil
}
func DownloadTor() (string, error) {
	downloadLink, version, err := GetLatestTorLinkAndVersion()
	core.LogDebug("Found TOR version: " + version)
	host.DeleteIfExists(host.GetInstallDir() + "tor.tar.gz")
	host.DeleteIfExists(host.GetInstallDir() + "tor")
	err = HttpGetFile(downloadLink, host.GetInstallDir()+"tor.tar.gz")
	if err != nil {
		return "", core.LogErrorReturn("Error downloading TOR: " + err.Error())
	}
	host.CreateFolder(host.GetInstallDir() + "tor")
	host.UntarFile(host.GetInstallDir()+"tor.tar.gz", host.GetInstallDir()+"tor/")
	if host.GetOS() == "darwin" {
		// Whitelist the TOR binary on macOS to bypass ASP security policy
		status, err := host.HelperCall("whitelist_tor")
		if err != nil {
			return "", core.LogErrorReturn("Could not whitelist TOR binary: " + err.Error())
		}
		if status == "success" {
			core.LogInfo("Tor binary whitelisted successfully")
			return host.GetInstallDir() + "tor/tor/YourPlaceTor", nil
		} else {
			core.LogError("Could not whitelist tor binary: " + status)
			return "", core.LogErrorReturn("Could not whitelist TOR binary: " + status)
		}
	}
	return host.GetInstallDir() + "tor/tor/YourPlaceTor", nil
}
func GetLatestTorBinarySignature() (string, error) {
	_, version, err := GetLatestTorLinkAndVersion()
	if err != nil {
		return "", core.LogDebugReturn("Error getting Tor version: " + err.Error())
	}
	arch, err := GetTorArchitecture()
	if err != nil {
		return "", core.LogDebugReturn("Error getting architecture: " + err.Error())
	}
	// Convert architecture format for signature URL
	var archString string
	if arch == "macOS (aarch64)" {
		archString = "macos-aarch64"
	} else if arch == "Windows (x86_64)" {
		archString = "windows-x86_64"
	} else {
		return "", core.LogDebugReturn("Unsupported architecture for signature: " + arch)
	}
	// Build signature URL based on example pattern
	signatureURL := "https://archive.torproject.org/tor-package-archive/torbrowser/" + version + "/tor-expert-bundle-" + archString + "-" + version + ".tar.gz.asc"
	resp, err := HttpGet(signatureURL, 20)
	if err != nil {
		return "", core.LogErrorReturn("Error downloading Tor signature: " + err.Error())
	}
	if resp == "" {
		return "", core.LogErrorReturn("Empty signature response")
	}
	return resp, nil
}
func GetTorArchitecture() (string, error) {
	arch := ""
	if host.GetOS() == "windows" {
		if host.GetCPUArch() == 64 && host.GetCPUVendor() == "intel" {
			arch = "Windows (x86_64)"
		}
	} else if host.GetOS() == "darwin" {
		if host.GetCPUArch() == 64 && host.GetCPUVendor() == "arm" {
			arch = "macOS (aarch64)"
		}
	}
	if arch == "" {
		return "", core.LogDebugReturn("Unsupported OS or architecture for Tor download")
	}
	return arch, nil
}
func GetLatestTorLinkAndVersion() (string, string, error) {
	resp, err := HttpGet("https://www.torproject.org/download/tor/", 20)
	if err != nil {
		return "", "", core.LogDebugReturn("Error reaching Tor download page: " + err.Error())
	}
	if resp == "" {
		return "", "", core.LogDebugReturn("Empty response from Tor download page")
	}
	arch, err := GetTorArchitecture()
	if err != nil {
		return "", "", core.LogDebugReturn("Unsupported Tor architecture: " + err.Error())
	}
	downloadLink, version, err := extractDownloadLink(resp, arch, false)
	return downloadLink, version, err
}

/* --- Helper Functions --- */
func extractDownloadLink(htmlContent, osArch string, useAlpha bool) (string, string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", "", core.LogDebugReturn("Failed to parse HTML: " + err.Error())
	}
	var findLink func(*html.Node) (string, string)
	findLink = func(n *html.Node) (string, string) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			var cells []*html.Node
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "td" {
					cells = append(cells, c)
				}
			}
			if len(cells) >= 3 {
				firstCellText := getTextContent(cells[0])
				if strings.Contains(firstCellText, osArch) {
					var targetCell *html.Node
					if useAlpha {
						targetCell = cells[2] // Alpha column
					} else {
						targetCell = cells[1] // Stable column
					}
					link := getHref(targetCell)
					if link != "" {
						version := extractVersionFromLink(link)
						return link, version
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if link, version := findLink(c); link != "" {
				return link, version
			}
		}
		return "", ""
	}
	link, version := findLink(doc)
	if link == "" {
		return "", "", core.LogDebugReturn("Tor download link not found")
	}
	return link, version, nil
}
func getTextContent(n *html.Node) string {
	var text strings.Builder
	var extractText func(*html.Node)
	extractText = func(node *html.Node) {
		if node.Type == html.TextNode {
			text.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			extractText(c)
		}
	}
	extractText(n)
	return strings.TrimSpace(text.String())
}
func getHref(cell *html.Node) string {
	var findHref func(*html.Node) string
	findHref = func(n *html.Node) string {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					return attr.Val
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if result := findHref(c); result != "" {
				return result
			}
		}
		return ""
	}
	return findHref(cell)
}
func extractVersionFromLink(link string) string {
	re := regexp.MustCompile(`tor-(\d+\.\d+\.\d+(?:\.\d+)?)`)
	matches := re.FindStringSubmatch(link)
	if len(matches) > 1 {
		return matches[1]
	}
	return "unknown"
}
