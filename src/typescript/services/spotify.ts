import "../../scss/components/spotify.scss";
import {HttpGetJson} from "../util/network";
import {ShowToast} from "../components/toast";

const SPOTIFY_API_URL = "https://api.spotify.com/v1";
const SPOTIFY_AUTH_URL = "https://accounts.spotify.com/authorize";
const SPOTIFY_EXPIRY_KEY = "spotifyTokenExpiry";
const SPOTIFY_IFRAME_API_URL = "https://open.spotify.com/embed/iframe-api/v1";
const SPOTIFY_REFRESH_KEY = "spotifyRefreshToken";
const SPOTIFY_SCOPES = "streaming user-read-private user-modify-playback-state";
const SPOTIFY_SDK_URL = "https://sdk.scdn.co/spotify-player.js";
const SPOTIFY_STATE_KEY = "spotifyAuthState";
const SPOTIFY_TOKEN_KEY = "spotifyAccessToken";
const SPOTIFY_TOKEN_URL = "https://accounts.spotify.com/api/token";
const SPOTIFY_URL_REGEX = /^https:\/\/open\.spotify\.com\/(track|playlist|album|episode|show)\/([a-zA-Z0-9]+)(?:\?.*)?$/;
const SPOTIFY_VERIFIER_KEY = "spotifyCodeVerifier";

let activeDeviceId: string = "";
let activePlayer: any = null;
let iframeScriptPromise: Promise<any> | null = null;
let premiumChecked: boolean = false;
let sdkScriptPromise: Promise<any> | null = null;

export function DisconnectSpotify(): void {
    localStorage.removeItem(SPOTIFY_TOKEN_KEY);
    localStorage.removeItem(SPOTIFY_REFRESH_KEY);
    localStorage.removeItem(SPOTIFY_EXPIRY_KEY);
    if (activePlayer) {
        try { activePlayer.disconnect(); } catch (_) {}
        activePlayer = null;
    }
    activeDeviceId = "";
    premiumChecked = false;
}
export async function GetSpotifyAccessToken(): Promise<string | null> {
    const token = localStorage.getItem(SPOTIFY_TOKEN_KEY);
    const expiryStr = localStorage.getItem(SPOTIFY_EXPIRY_KEY);
    if (!token || !expiryStr) return null;
    const expiry = parseInt(expiryStr, 10);
    if (Date.now() < expiry - 60000) return token;
    return await refreshSpotifyToken();
}
export function IsSpotifyConnected(): boolean {
    return !!localStorage.getItem(SPOTIFY_TOKEN_KEY);
}
export function IsValidSpotifyUrl(url: string): string | null {
    if (!url) return null;
    const match = url.match(SPOTIFY_URL_REGEX);
    if (!match) return null;
    return `spotify:${match[1]}:${match[2]}`;
}
export function MountSpotifyConnectPill(container: HTMLElement, onConnected: () => void): void {
    container.innerHTML = "";
    container.classList.remove("hidden");
    const img = document.createElement("img");
    img.src = "/static/image/spotify.svg";
    img.alt = "Spotify";
    const label = document.createElement("span");
    label.textContent = "Connect Spotify";
    container.appendChild(img);
    container.appendChild(label);
    container.onclick = async () => {
        const ok = await StartSpotifyAuth();
        if (ok) onConnected();
    };
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
export async function MountSpotifyFullPlayer(container: HTMLElement, spotifyUrl: string): Promise<boolean> {
    const uri = IsValidSpotifyUrl(spotifyUrl);
    if (!uri) return false;
    const parts = uri.split(":");
    if (parts.length !== 3 || parts[1] !== "track") return false;
    const trackId = parts[2];
    if (!premiumChecked) {
        const premium = await checkSpotifyPremium();
        premiumChecked = true;
        if (!premium) {
            ShowToast("Spotify Premium is required for full-track playback");
            return false;
        }
    }
    const meta = await fetchTrackMetadata(trackId);
    if (!meta) return false;
    const Spotify = await loadSpotifySdk();
    if (!Spotify) return false;
    if (!activePlayer) {
        activePlayer = new Spotify.Player({
            name: "YourPlace",
            getOAuthToken: (cb: (t: string) => void) => {
                GetSpotifyAccessToken().then((t) => { if (t) cb(t); });
            },
            volume: 0.5,
        });
        activePlayer.addListener("ready", ({device_id}: {device_id: string}) => {
            activeDeviceId = device_id;
        });
        activePlayer.addListener("not_ready", () => {
            activeDeviceId = "";
        });
        activePlayer.addListener("authentication_error", () => { DisconnectSpotify(); });
        activePlayer.addListener("account_error", () => { ShowToast("Spotify Premium is required"); });
        const connected = await activePlayer.connect();
        if (!connected) {
            activePlayer = null;
            return false;
        }
    }
    let waited = 0;
    while (!activeDeviceId && waited < 5000) {
        await new Promise((r) => setTimeout(r, 100));
        waited += 100;
    }
    if (!activeDeviceId) return false;
    container.innerHTML = "";
    const player = document.createElement("div");
    player.id = "spotifyFullPlayer";
    const imgUrl = meta.album?.images?.[0]?.url || "";
    const safeImgUrl = validateSpotifyImageUrl(imgUrl);
    if (safeImgUrl) {
        const img = document.createElement("img");
        img.className = "spotifyAlbumArt";
        img.src = safeImgUrl;
        img.alt = "";
        player.appendChild(img);
    }
    const info = document.createElement("div");
    info.className = "spotifyTrackInfo";
    const name = document.createElement("div");
    name.className = "spotifyTrackName";
    name.textContent = meta.name || "";
    info.appendChild(name);
    const artistNames = (meta.artists || []).map((a: any) => a.name).join(", ");
    const artist = document.createElement("div");
    artist.className = "spotifyTrackArtist";
    artist.textContent = artistNames;
    info.appendChild(artist);
    const progressRow = document.createElement("div");
    progressRow.className = "spotifyProgressRow";
    const timeCurrent = document.createElement("span");
    timeCurrent.className = "spotifyTimeCurrent";
    timeCurrent.textContent = "0:00";
    const progressBar = document.createElement("div");
    progressBar.className = "spotifyProgressBar";
    const progressFill = document.createElement("div");
    progressFill.className = "spotifyProgressFill";
    progressBar.appendChild(progressFill);
    const timeTotal = document.createElement("span");
    timeTotal.className = "spotifyTimeTotal";
    timeTotal.textContent = formatTime(meta.duration_ms || 0);
    progressRow.appendChild(timeCurrent);
    progressRow.appendChild(progressBar);
    progressRow.appendChild(timeTotal);
    info.appendChild(progressRow);
    const controls = document.createElement("div");
    controls.className = "spotifyControls";
    const playBtn = document.createElement("button");
    playBtn.className = "spotifyPlayPauseBtn";
    const playIcon = document.createElement("i");
    playIcon.className = "bi bi-pause-fill";
    playBtn.appendChild(playIcon);
    const disconnectBtn = document.createElement("button");
    disconnectBtn.className = "spotifyDisconnectBtn";
    disconnectBtn.textContent = "Disconnect";
    controls.appendChild(playBtn);
    controls.appendChild(disconnectBtn);
    info.appendChild(controls);
    player.appendChild(info);
    container.appendChild(player);
    const playResp = await spotifyApi("PUT", "/me/player/play?device_id=" + encodeURIComponent(activeDeviceId), {uris: [uri]});
    if (!playResp || (!playResp.ok && playResp.status !== 204)) {
        container.innerHTML = "";
        return false;
    }
    playBtn.onclick = async () => {
        if (activePlayer) await activePlayer.togglePlay();
    };
    disconnectBtn.onclick = () => {
        DisconnectSpotify();
        window.location.reload();
    };
    progressBar.onclick = async (e) => {
        const rect = progressBar.getBoundingClientRect();
        const ratio = (e.clientX - rect.left) / rect.width;
        const duration = meta.duration_ms || 0;
        const pos = Math.floor(ratio * duration);
        if (activePlayer) await activePlayer.seek(pos);
    };
    const stateListener = (state: any) => {
        if (!state) return;
        const pct = state.duration > 0 ? (state.position / state.duration) * 100 : 0;
        progressFill.style.width = pct + "%";
        timeCurrent.textContent = formatTime(state.position);
        playIcon.className = state.paused ? "bi bi-play-fill" : "bi bi-pause-fill";
    };
    activePlayer.addListener("player_state_changed", stateListener);
    const positionTimer = setInterval(async () => {
        if (activePlayer) {
            const s = await activePlayer.getCurrentState();
            if (s) stateListener(s);
        }
    }, 1000);
    const observer = new MutationObserver(() => {
        if (!document.body.contains(player)) {
            clearInterval(positionTimer);
            observer.disconnect();
        }
    });
    observer.observe(document.body, {childList: true, subtree: true});
    return true;
}
export async function StartSpotifyAuth(): Promise<boolean> {
    const clientId = await fetchClientId();
    if (!clientId) {
        ShowToast("Spotify is not configured on this server");
        return false;
    }
    const verifier = generateRandomString(64);
    const challenge = base64urlEncode(await sha256(verifier));
    const state = generateRandomString(32);
    sessionStorage.setItem(SPOTIFY_VERIFIER_KEY, verifier);
    sessionStorage.setItem(SPOTIFY_STATE_KEY, state);
    const redirectUri = window.location.origin + "/services/spotify/callback";
    const authUrl = SPOTIFY_AUTH_URL +
        "?response_type=code" +
        "&client_id=" + encodeURIComponent(clientId) +
        "&redirect_uri=" + encodeURIComponent(redirectUri) +
        "&code_challenge_method=S256" +
        "&code_challenge=" + challenge +
        "&state=" + state +
        "&scope=" + encodeURIComponent(SPOTIFY_SCOPES);
    const popup = window.open(authUrl, "spotifyAuth", "width=500,height=700");
    if (!popup) {
        ShowToast("Popup blocked — please allow popups for Spotify login");
        return false;
    }
    return await new Promise<boolean>((resolve) => {
        let checker: ReturnType<typeof setInterval>;
        const listener = async (ev: MessageEvent) => {
            if (ev.origin !== window.location.origin) return;
            if (!ev.data || ev.data.source !== "spotifyAuth") return;
            window.removeEventListener("message", listener);
            clearInterval(checker);
            const {code, state: returnedState, error} = ev.data;
            const savedState = sessionStorage.getItem(SPOTIFY_STATE_KEY);
            const savedVerifier = sessionStorage.getItem(SPOTIFY_VERIFIER_KEY);
            sessionStorage.removeItem(SPOTIFY_STATE_KEY);
            sessionStorage.removeItem(SPOTIFY_VERIFIER_KEY);
            if (error || !code || !returnedState || returnedState !== savedState || !savedVerifier) {
                ShowToast("Spotify login failed");
                resolve(false);
                return;
            }
            const body = new URLSearchParams({
                grant_type: "authorization_code",
                code: code,
                redirect_uri: redirectUri,
                client_id: clientId,
                code_verifier: savedVerifier,
            });
            try {
                const resp = await fetch(SPOTIFY_TOKEN_URL, {
                    method: "POST",
                    headers: {"Content-Type": "application/x-www-form-urlencoded"},
                    body: body.toString(),
                });
                if (!resp.ok) {
                    ShowToast("Spotify login failed");
                    resolve(false);
                    return;
                }
                const data = await resp.json();
                persistTokens(data.access_token, data.refresh_token, data.expires_in);
                resolve(true);
            } catch (_) {
                ShowToast("Spotify login failed");
                resolve(false);
            }
        };
        window.addEventListener("message", listener);
        checker = setInterval(() => {
            if (popup.closed) {
                clearInterval(checker);
                window.removeEventListener("message", listener);
                resolve(false);
            }
        }, 500);
    });
}

function base64urlEncode(buffer: ArrayBuffer): string {
    const bytes = new Uint8Array(buffer);
    let binary = "";
    for (let i = 0; i < bytes.length; i++) {
        binary += String.fromCharCode(bytes[i]);
    }
    return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}
async function checkSpotifyPremium(): Promise<boolean> {
    const resp = await spotifyApi("GET", "/me");
    if (!resp || !resp.ok) return false;
    const data = await resp.json();
    return data.product === "premium";
}
async function fetchClientId(): Promise<string | null> {
    const resp = await HttpGetJson("/services/spotify/clientid");
    if (resp[0] !== 200) return null;
    const id = resp[1]?.clientId;
    if (!id) return null;
    return id;
}
async function fetchTrackMetadata(trackId: string): Promise<any | null> {
    const resp = await spotifyApi("GET", "/tracks/" + encodeURIComponent(trackId));
    if (!resp || !resp.ok) return null;
    return await resp.json();
}
function formatTime(ms: number): string {
    const s = Math.floor(ms / 1000);
    const m = Math.floor(s / 60);
    const r = s % 60;
    return `${m}:${r < 10 ? "0" : ""}${r}`;
}
function generateRandomString(length: number): string {
    const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789";
    const random = new Uint8Array(length);
    window.crypto.getRandomValues(random);
    let result = "";
    for (let i = 0; i < length; i++) {
        result += chars[random[i] % chars.length];
    }
    return result;
}
function loadSpotifyIframeApi(): Promise<any> {
    if (iframeScriptPromise) return iframeScriptPromise;
    iframeScriptPromise = new Promise((resolve) => {
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
    return iframeScriptPromise;
}
async function loadSpotifySdk(): Promise<any> {
    if (sdkScriptPromise) return sdkScriptPromise;
    sdkScriptPromise = new Promise((resolve) => {
        if ((window as any).Spotify) {
            resolve((window as any).Spotify);
            return;
        }
        (window as any).onSpotifyWebPlaybackSDKReady = () => {
            resolve((window as any).Spotify);
        };
        const script = document.createElement("script");
        script.src = SPOTIFY_SDK_URL;
        script.async = true;
        document.head.appendChild(script);
    });
    return sdkScriptPromise;
}
function persistTokens(access: string, refresh: string, expiresInSeconds: number): void {
    localStorage.setItem(SPOTIFY_TOKEN_KEY, access);
    localStorage.setItem(SPOTIFY_REFRESH_KEY, refresh);
    localStorage.setItem(SPOTIFY_EXPIRY_KEY, String(Date.now() + expiresInSeconds * 1000));
}
async function refreshSpotifyToken(): Promise<string | null> {
    const refreshToken = localStorage.getItem(SPOTIFY_REFRESH_KEY);
    if (!refreshToken) {
        DisconnectSpotify();
        return null;
    }
    const clientId = await fetchClientId();
    if (!clientId) return null;
    const body = new URLSearchParams({
        grant_type: "refresh_token",
        refresh_token: refreshToken,
        client_id: clientId,
    });
    try {
        const resp = await fetch(SPOTIFY_TOKEN_URL, {
            method: "POST",
            headers: {"Content-Type": "application/x-www-form-urlencoded"},
            body: body.toString(),
        });
        if (!resp.ok) {
            DisconnectSpotify();
            return null;
        }
        const data = await resp.json();
        persistTokens(data.access_token, data.refresh_token || refreshToken, data.expires_in);
        return data.access_token;
    } catch (_) {
        return null;
    }
}
async function sha256(plain: string): Promise<ArrayBuffer> {
    const encoder = new TextEncoder();
    const data = encoder.encode(plain);
    return window.crypto.subtle.digest("SHA-256", data);
}
async function spotifyApi(method: string, path: string, body?: any): Promise<Response | null> {
    const token = await GetSpotifyAccessToken();
    if (!token) return null;
    const headers: Record<string, string> = {"Authorization": "Bearer " + token};
    if (body !== undefined) headers["Content-Type"] = "application/json";
    try {
        return await fetch(SPOTIFY_API_URL + path, {
            method: method,
            headers: headers,
            body: body !== undefined ? JSON.stringify(body) : undefined,
        });
    } catch (_) {
        return null;
    }
}
function validateSpotifyImageUrl(url: string): string {
    try {
        const parsed = new URL(url);
        if (parsed.protocol !== "https:") return "";
        if (!parsed.hostname.endsWith(".scdn.co")) return "";
        return parsed.toString();
    } catch (_) {
        return "";
    }
}
