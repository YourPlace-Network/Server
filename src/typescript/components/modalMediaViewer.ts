window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import {XSSSanitizeUrl} from "../util/security";
import "../../scss/components/modalMediaViewer.scss";

function cleanupModalMediaViewer(mediaDiv: HTMLDivElement) {
    const videos = mediaDiv.querySelectorAll("video") as NodeListOf<HTMLVideoElement>;
    videos.forEach((video: HTMLVideoElement) => {
        video.pause();
        video.currentTime = 0;
    });
    mediaDiv.innerHTML = "";
}

function shouldKeepModalMediaViewerOpen(target: HTMLElement): boolean {
    return !!target.closest(".attachmentGrid, .btn-close, .carousel-control-next, .carousel-control-prev, .carousel-indicators, .image-container, .modalMediaViewerStage, iframe, img, video");
}

function initializeModalMediaViewer(modalMediaViewer: HTMLDivElement, mediaDiv: HTMLDivElement) {
    if (modalMediaViewer.dataset.initialized === "true") {
        return;
    }
    modalMediaViewer.dataset.initialized = "true";
    const modalBody = modalMediaViewer.querySelector(".modal-body") as HTMLDivElement | null;
    modalMediaViewer.addEventListener("hide.bs.modal", () => {
        if (modalMediaViewer.contains(document.activeElement)) {
            (document.activeElement as HTMLElement).blur();
        }
    });
    modalMediaViewer.addEventListener("hidden.bs.modal", () => {
        cleanupModalMediaViewer(mediaDiv);
    });
    if (modalBody) {
        modalBody.addEventListener("click", (event: MouseEvent) => {
            const target = event.target as HTMLElement | null;
            if (!target || shouldKeepModalMediaViewerOpen(target)) {
                return;
            }
            if (target.closest("#mediaDiv, .modal-body")) {
                const modal = window.bootstrap.Modal.getOrCreateInstance(modalMediaViewer);
                modal.hide();
            }
        });
    }
}

function isVideoMediaUrl(mediaUrl: string): boolean {
    const normalizedMediaUrl = mediaUrl.split(/[?#]/)[0].toLowerCase();
    return normalizedMediaUrl.endsWith(".mov") ||
        normalizedMediaUrl.endsWith(".mp4") ||
        normalizedMediaUrl.endsWith(".ogg") ||
        normalizedMediaUrl.endsWith(".webm");
}

function createAvatarViewerContent(mediaUrl: string, altText: string): HTMLDivElement | null {
    const sanitizedMediaUrl = XSSSanitizeUrl(mediaUrl);
    if (sanitizedMediaUrl === "#") {
        return null;
    }
    const wrapper = document.createElement("div") as HTMLDivElement;
    wrapper.classList.add("modalMediaViewerStage");
    if (isVideoMediaUrl(mediaUrl)) {
        const video = document.createElement("video") as HTMLVideoElement;
        video.controls = true;
        video.playsInline = true;
        video.preload = "metadata";
        video.src = sanitizedMediaUrl;
        video.setAttribute("aria-label", altText);
        video.setAttribute("referrerpolicy", "no-referrer");
        wrapper.appendChild(video);
        return wrapper;
    }
    const image = document.createElement("img") as HTMLImageElement;
    image.alt = altText;
    image.crossOrigin = "anonymous";
    image.referrerPolicy = "no-referrer";
    image.src = sanitizedMediaUrl;
    wrapper.appendChild(image);
    return wrapper;
}

export function ShowModalMediaViewer(element: HTMLElement) {
    const modalMediaViewer = document.getElementById("modalMediaViewer") as HTMLDivElement | null;
    const mediaDiv = document.getElementById("mediaDiv") as HTMLDivElement | null;
    if (!modalMediaViewer || !mediaDiv) {
        return;
    }
    initializeModalMediaViewer(modalMediaViewer, mediaDiv);
    cleanupModalMediaViewer(mediaDiv);
    mediaDiv.appendChild(element);
    const modal = window.bootstrap.Modal.getOrCreateInstance(modalMediaViewer);
    modal.show();
}

export function ShowAvatarMediaViewer(mediaUrl: string, altText: string = "avatar") {
    const avatarViewerContent = createAvatarViewerContent(mediaUrl, altText);
    if (!avatarViewerContent) {
        return;
    }
    ShowModalMediaViewer(avatarViewerContent);
}
