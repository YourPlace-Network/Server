window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/pages/faq.scss";
import "../components/menu";
import {ExpandAccordionByHash} from "../util/bootstrap";

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        document.addEventListener("shown.bs.collapse", function (event) {
            const accordionItem = event.target as HTMLElement;
            const yOffset = -60;
            const y = accordionItem.getBoundingClientRect().top + window.scrollY + yOffset;
            window.scrollTo({top: y, behavior: "smooth"});
        });
        ExpandAccordionByHash();
    }
})();