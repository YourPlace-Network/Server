import {LogError} from "../util/log";

window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/toast.scss"
import {HttpGetJson} from "../util/network";
import {XSSSanitizeTextUrl} from "../util/security";

// HTML Template:  {{template "toast" .}}

export async function GetToasts() {
    try {
        // Add timeout for toast fetching to prevent blocking
        const timeoutPromise = new Promise((_, reject) => {
            setTimeout(() => reject(new Error("Toast fetch timeout")), 5000);
        });
        const fetchPromise = HttpGetJson("/notification/toasts");
        const result = await Promise.race([fetchPromise, timeoutPromise]) as any;
        if (result && result[1] && result[1]["toasts"]) {
            ShowToast(result[1]["toasts"]);
        }
    } catch (error) {
        LogError("Failed to fetch toasts (non-critical): " + error);
    }
}
function CreateToast(autohide: boolean = false, delay: number = 2000, showCloseBtn: boolean = true): HTMLDivElement {
    let toastDiv = document.createElement("div");
    toastDiv.className = "toast hide";
    toastDiv.setAttribute("role", "alert");
    toastDiv.setAttribute("aria-live", "assertive");
    toastDiv.setAttribute("aria-atomic", "true");
    toastDiv.setAttribute("data-bs-autohide", autohide ? "true" : "false");
    if (autohide) {
        toastDiv.setAttribute("data-bs-delay", delay.toString());
    }
    let toastFlex = document.createElement("div");
    toastFlex.className = "d-flex";
    let toastBody = document.createElement("div");
    toastBody.className = "toast-body";
    toastFlex.appendChild(toastBody);
    if (showCloseBtn) {
        let toastCloseBtn = document.createElement("button");
        toastCloseBtn.type = "button";
        toastCloseBtn.className = "btn-close me-2 m-auto";
        toastCloseBtn.setAttribute("data-bs-dismiss", "toast");
        toastCloseBtn.setAttribute("aria-label", "Close");
        toastCloseBtn.innerHTML = "";
        let closeIcon = document.createElement("i");
        closeIcon.className = "bi bi-x";
        toastCloseBtn.appendChild(closeIcon);
        toastFlex.appendChild(toastCloseBtn);
    }
    toastDiv.appendChild(toastFlex);
    return toastDiv;
}
export function ShowToast(message: string) {
    let toastDiv = CreateToast();
    toastDiv.children[0].children[0].innerHTML = XSSSanitizeTextUrl(message);
    document.getElementById("toastContainer")!.appendChild(toastDiv);
    let toast = new window.bootstrap.Toast(toastDiv, {});
    toast.show();
}
export function ShowSavedToast() {
    let toastDiv = CreateToast(true, 2000, false);
    toastDiv.children[0].children[0].textContent = "Setting Saved!";
    document.getElementById("toastContainer")!.appendChild(toastDiv);
    let toast = new window.bootstrap.Toast(toastDiv, {});
    toast.show();
}