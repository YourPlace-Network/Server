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
            const localWalletData = localStorage.getItem("yp_local_wallet_ethereum");
            localStorage.clear();
            if (localWalletData) {
                localStorage.setItem("yp_local_wallet_ethereum", localWalletData);
            }
            window.location.replace("/");
        }

        DisconnectWallet().then();
    }
})();