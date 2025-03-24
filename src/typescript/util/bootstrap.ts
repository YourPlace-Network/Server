export function ExpandAccordionByHash() {
    if (!window.location.hash) return;
    const targetElement = document.querySelector(window.location.hash);
    if (!targetElement) return;
    // Find all parent accordions
    let element: Element | null = targetElement;
    while (element && element !== document.body) {
        // If this element is a collapse panel
        if (element.classList.contains("accordion-collapse")) {
            // Show this accordion panel
            const collapse = new window.bootstrap.Collapse(element, {
                toggle: false
            });
            collapse.show();
            // Also need to find any parent accordions
            const parentAccordion = element.closest(".accordion");
            if (parentAccordion) {
                const parentCollapse = parentAccordion.querySelector(".accordion-collapse");
                if (parentCollapse) {
                    const collapse = new window.bootstrap.Collapse(parentCollapse, {
                        toggle: false
                    });
                    collapse.show();
                }
            }
        }
        element = element.parentElement;
    }
}
export function InitTooltips() {
    let tooltipTriggerList = [].slice.call(document.querySelectorAll("[data-bs-toggle=\"tooltip\"]"));
    tooltipTriggerList.map(function (tooltipTriggerEl) {return new window.bootstrap.Tooltip(tooltipTriggerEl)});
}