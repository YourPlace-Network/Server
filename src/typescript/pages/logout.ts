window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import {DisconnectWallet} from "../util/blockchain/wallet";

declare global { // Extend the window interface with public callback objects
    interface Window {
        DisconnectWalletCallback: () => void;
    }
}
(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        window.DisconnectWalletCallback = function () {
            const localWallets: Record<string, string> = {};
            for (let i = 0; i < localStorage.length; i++) {
                const key = localStorage.key(i);
                if (key?.startsWith("yp_local_wallet_")) {
                    localWallets[key] = localStorage.getItem(key)!;
                }
            }
            localStorage.clear();
            for (const [key, value] of Object.entries(localWallets)) {
                localStorage.setItem(key, value);
            }
            window.location.replace("/");
        }

        DisconnectWallet().then();
    }
})();