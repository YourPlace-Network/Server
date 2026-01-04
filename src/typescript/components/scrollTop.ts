window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/scrollTop.scss"

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        function hideScrollTop() {
            document.getElementById("scrollTop")!.style.display = "none";
        }
        function showScrollTop() {
            document.getElementById("scrollTop")!.style.display = "block";
        }

        document.getElementById("scrollTop")!.addEventListener("click", function () {
            window.scrollTo({ top: 0, behavior: "smooth" });
        });

        window.onscroll = function scroll() {
            if (window.scrollY >= 400 ) {
                showScrollTop();
            } else {
                hideScrollTop();
            }
        }
    }
})();
