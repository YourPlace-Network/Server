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

            // Update the associated button state
            const accordionButton = document.querySelector(`[data-bs-target="${element.id}"]`);
            if (accordionButton) {
                accordionButton.classList.remove("collapsed");
                accordionButton.setAttribute("aria-expanded", "true");
            }
        }
        element = element.parentElement;
    }
}
export function InitTooltips() {
    let tooltipTriggerList = [].slice.call(document.querySelectorAll("[data-bs-toggle=\"tooltip\"]"));
    tooltipTriggerList.map(function (tooltipTriggerEl) {return new window.bootstrap.Tooltip(tooltipTriggerEl)});
}