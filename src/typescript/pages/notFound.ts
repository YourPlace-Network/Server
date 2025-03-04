window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/pages/notFound.scss";
import {Sleep} from "../util/time";

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        var count = 10;
        let DOM = {
            countdown: document.getElementById('countdown') as HTMLSpanElement,
        };

        async function load() {
            for (var i = count; i >= 0; i--) {
                if (i === 0) {
                    window.location.replace("/");
                    break;
                }
                (DOM.countdown).textContent = i.toString();
                await Sleep(1000);
            }
        }

        load().then();
    }
})();