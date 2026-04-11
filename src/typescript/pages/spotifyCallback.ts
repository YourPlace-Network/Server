(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        const params = new URLSearchParams(window.location.search);
        const code = params.get("code");
        const state = params.get("state");
        const error = params.get("error");
        const messageDiv = document.getElementById("spotifyCallbackMessage");
        if (!window.opener) {
            if (messageDiv) messageDiv.textContent = "This page should be opened from the Spotify connect flow. You can close this window.";
            return;
        }
        try {
            window.opener.postMessage({
                source: "spotifyAuth",
                code: code,
                state: state,
                error: error,
            }, window.location.origin);
        } catch (_) {}
        window.close();
    }
})();
