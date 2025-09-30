window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/toast.scss"
import {XSSSanitizeTextUrl} from "../util/security";
import {DismissNotification, type Notification} from "../util/notifications";

// HTML Template:  {{template "toast" .}}

export function ShowToastNotification(notification: Notification) {
    let toastDiv = CreateToast(notification);
    document.getElementById("toastContainer")!.appendChild(toastDiv);
    let toast = new window.bootstrap.Toast(toastDiv, {});
    toast.show();
}
export function CreateToast(notification: Notification): HTMLDivElement {
    let toastDiv = document.createElement("div");
    toastDiv.className = "toast hide";
    toastDiv.setAttribute("role", "alert");
    toastDiv.setAttribute("aria-live", "assertive");
    toastDiv.setAttribute("aria-atomic", "true");
    toastDiv.setAttribute("data-bs-autohide", notification.dismissable ? "false" : "true");
    toastDiv.setAttribute("data-notification-uid", notification.uid);
    if (!notification.dismissable) {
        toastDiv.setAttribute("data-bs-delay", "4000");
    }
    let toastFlex = document.createElement("div");
    toastFlex.className = "d-flex";
    let toastBody = document.createElement("div");
    toastBody.className = "toast-body";
    toastBody.innerHTML = XSSSanitizeTextUrl(notification.message);
    toastFlex.appendChild(toastBody);
    if (notification.dismissable) {
        let toastCloseBtn = document.createElement("button");
        toastCloseBtn.type = "button";
        toastCloseBtn.className = "btn-close me-2 m-auto";
        toastCloseBtn.setAttribute("data-bs-dismiss", "toast");
        toastCloseBtn.setAttribute("aria-label", "Close");
        toastCloseBtn.onclick = async () => {
            await DismissNotification(notification.uid);
        };
        let closeIcon = document.createElement("i");
        closeIcon.className = "bi bi-x";
        toastCloseBtn.appendChild(closeIcon);
        toastFlex.appendChild(toastCloseBtn);
    }
    toastDiv.appendChild(toastFlex);
    return toastDiv;
}
export function ShowToast(message: string) {
    const notification: Notification = {
        uid: `manual_${Date.now()}`,
        type: "manual",
        message: message,
        dismissable: false
    };
    ShowToastNotification(notification);
}
export function ShowSavedToast() {
    const notification: Notification = {
        uid: `saved_${Date.now()}`,
        type: "manual",
        message: "Setting Saved!",
        dismissable: false
    };
    ShowToastNotification(notification);
}