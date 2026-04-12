(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        if (window.location.hostname === "127.0.0.1") {
            const normalized = window.location.protocol + "//localhost" +
                (window.location.port ? ":" + window.location.port : "") +
                window.location.pathname + window.location.search;
            window.location.replace(normalized);
            return;
        }
        const params = new URLSearchParams(window.location.search);
        const payload = {
            source: "spotifyAuth",
            code: params.get("code"),
            state: params.get("state"),
            error: params.get("error"),
        };
        try {
            const channel = new BroadcastChannel("yp_spotifyAuth");
            channel.postMessage(payload);
            channel.close();
        } catch (_) {}
        try {
            if (window.opener) {
                window.opener.postMessage(payload, "*");
            }
        } catch (_) {}
        window.close();
    }
})();
