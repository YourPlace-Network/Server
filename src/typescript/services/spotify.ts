const SPOTIFY_IFRAME_API_URL = "https://open.spotify.com/embed/iframe-api/v1";
const SPOTIFY_URL_REGEX = /^https:\/\/open\.spotify\.com\/(track|playlist|album|episode|show)\/([a-zA-Z0-9]+)(?:\?.*)?$/;

let scriptPromise: Promise<any> | null = null;

export function IsValidSpotifyUrl(url: string): string | null {
    if (!url) return null;
    const match = url.match(SPOTIFY_URL_REGEX);
    if (!match) return null;
    return `spotify:${match[1]}:${match[2]}`;
}

function loadSpotifyIframeApi(): Promise<any> {
    if (scriptPromise) return scriptPromise;
    scriptPromise = new Promise((resolve) => {
        if ((window as any).SpotifyIframeApi) {
            resolve((window as any).SpotifyIframeApi);
            return;
        }
        (window as any).onSpotifyIframeApiReady = (IFrameAPI: any) => {
            (window as any).SpotifyIframeApi = IFrameAPI;
            resolve(IFrameAPI);
        };
        const script = document.createElement("script");
        script.src = SPOTIFY_IFRAME_API_URL;
        script.async = true;
        document.head.appendChild(script);
    });
    return scriptPromise;
}

export async function MountSpotifyEmbed(container: HTMLElement, spotifyUrl: string): Promise<void> {
    const uri = IsValidSpotifyUrl(spotifyUrl);
    if (!uri) return;
    container.innerHTML = "";
    const placeholder = document.createElement("div");
    container.appendChild(placeholder);
    const IFrameAPI = await loadSpotifyIframeApi();
    IFrameAPI.createController(placeholder, {uri: uri, width: "100%", height: 152}, (ctrl: any) => {
        try {
            ctrl.play();
        } catch (_) {}
    });
}
