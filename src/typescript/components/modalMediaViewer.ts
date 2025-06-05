window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");


export function ShowModalMediaViewer(element: HTMLDivElement) {
    const modalMediaViewer = document.getElementById("modalMediaViewer") as HTMLDivElement;
    const mediaDiv = document.getElementById("mediaDiv") as HTMLDivElement;
    mediaDiv.appendChild(element);
    const modal = new window.bootstrap.Modal(modalMediaViewer, {});
    modal.show();
}