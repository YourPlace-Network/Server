window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/modalMediaViewer.scss";

export function ShowModalMediaViewer(element: HTMLDivElement) {
    const modalMediaViewer = document.getElementById("modalMediaViewer") as HTMLDivElement;
    const mediaDiv = document.getElementById("mediaDiv") as HTMLDivElement;

    // Clear previous content
    mediaDiv.innerHTML = '';

    mediaDiv.appendChild(element);
    const modal = new window.bootstrap.Modal(modalMediaViewer, {});

    // Clear content when modal is hidden
    modalMediaViewer.addEventListener('hidden.bs.modal', () => {
        mediaDiv.innerHTML = '';
    }, { once: true });

    modal.show();
}