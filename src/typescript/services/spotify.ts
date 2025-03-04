export function SpotifyEmbed(s: string): string | null {
    let match = s.match(/https:\/\/open\.spotify\.com\/track\/([a-zA-Z0-9]+)/);
    if (match && match[1]) {
        const trackId = match[1];
        return `<iframe src="https://open.spotify.com/embed/track/${trackId}" width="300" height="380" frameborder="0" allowtransparency="true" allow="encrypted-media"></iframe>`;
    }
    match = s.match(/https:\/\/open\.spotify\.com\/playlist\/([a-zA-Z0-9]+)/);
    if (match && match[1]) {
        const playlistId = match[1];
        return `<iframe src="https://open.spotify.com/embed/playlist/${playlistId}" width="300" height="380" frameborder="0" allowtransparency="true" allow="encrypted-media"></iframe>`;
    }
    return null;
}

// Test the function
//const text = "Here's a Spotify track you might like: https://open.spotify.com/track/0N4Mnow5PYo0NALjuySiGe?si=ee9e6ae6ddfa4373";
//const embedCode = SpotifyEmbed(text);
