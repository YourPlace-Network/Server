window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/pages/faq.scss";
import "../components/menu";
import {ExpandAccordionByHash} from "../util/bootstrap";

const DOM = {} as {
    accordionFAQ: HTMLDivElement
};

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        DOM.accordionFAQ = document.getElementById("accordionFAQ")! as HTMLDivElement;
        document.addEventListener("shown.bs.collapse", function (event) {
            const collapseElement = event.target as HTMLElement;
            if (!DOM.accordionFAQ.contains(collapseElement)) return;
            window.history.replaceState(null, "", `${window.location.pathname}${window.location.search}#${collapseElement.id}`);
            const accordionItem = collapseElement.closest(".accordion-item") as HTMLElement | null;
            if (accordionItem === null) return;
            const yOffset = -60;
            const y = accordionItem.getBoundingClientRect().top + window.scrollY + yOffset;
            window.scrollTo({top: y, behavior: "smooth"});
        });
        document.addEventListener("hidden.bs.collapse", function (event) {
            const collapseElement = event.target as HTMLElement;
            if (!DOM.accordionFAQ.contains(collapseElement)) return;
            if (window.location.hash !== `#${collapseElement.id}`) return;
            window.history.replaceState(null, "", `${window.location.pathname}${window.location.search}`);
        });
        window.addEventListener("hashchange", ExpandAccordionByHash);
        ExpandAccordionByHash();
    }
})();
