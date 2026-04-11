package services

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

func CreateSpotifyEmbed(spotifyUri string) (string, error) {
	isValid, spotifyUri := IsValidSpotifyUri(spotifyUri)
	if isValid {
		embedScriptFormatStr := `window.onSpotifyIframeApiReady = (IFrameAPI) => {
			  let element = document.getElementById('embed-iframe');
			  let options = {
				  uri: '%s'
				};
			  let callback = (EmbedController) => {};
			  IFrameAPI.createController(element, options, callback);
			};`
		embed := fmt.Sprintf(embedScriptFormatStr, spotifyUri)
		return embed, nil
	}
	return "", errors.New("invalid Spotify URI")
}
func GetSpotifyClientID(databaseClientID string) string {
	if envClientID := os.Getenv("YOURPLACE_SPOTIFY_CLIENT_ID"); envClientID != "" {
		return envClientID
	}
	return databaseClientID
}
func IsSpotifyClientIDEnvLocked() bool {
	return os.Getenv("YOURPLACE_SPOTIFY_CLIENT_ID") != ""
}
func IsValidSpotifyClientID(clientID string) bool {
	if len(clientID) != 32 {
		return false
	}
	for _, r := range clientID {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}
func IsValidSpotifyUri(uri string) (bool, string) {
	if strings.HasPrefix(uri, "spotify:") {
		return true, uri
	}
	if strings.HasPrefix(uri, "https://open.spotify.com/") {
		parsedURL, err := url.Parse(uri)
		if err != nil {
			return false, ""
		}
		spotifyPath := strings.ReplaceAll(parsedURL.Path, "/", ":")
		spotifyPath = "spotify" + spotifyPath
		return true, spotifyPath
	}
	return false, ""
}
