window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/toast.scss"
import {LogError} from "../util/log";
import {HttpGetJson, HttpPostJson} from "../util/network";
import {XSSSanitizeTextUrl} from "../util/security";

// HTML Template:  {{template "toast" .}}

interface NotificationObject {
    uid: string;
    type: string;
    message: string;
    dismissable: boolean;
}

export async function GetNotifications() {
    try {
        const timeoutPromise = new Promise((_, reject) => {
            setTimeout(() => reject(new Error("Notification fetch timeout")), 5000);
        });
        const fetchPromise = HttpGetJson("/notification");
        const result = await Promise.race([fetchPromise, timeoutPromise]) as any;
        if (result && result[1] && result[1]["notifications"]) {
            DispatchNotifications(result[1]["notifications"]);
        }
    } catch (error) {
        LogError("Failed to fetch notifications (non-critical): " + error);
    }
}

function DispatchNotifications(notifications: NotificationObject[]) {
    for (const notification of notifications) {
        if (notification.type === "user" || notification.type === "system") {
            ShowToastNotification(notification);
        }
    }
}
function CreateToast(notification: NotificationObject): HTMLDivElement {
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
async function DismissNotification(uid: string) {
    try {
        await HttpPostJson(`/notification/dismiss/${uid}`, {}, "");
    } catch (error) {
        LogError("Failed to dismiss notification: " + error);
    }
}

function ShowToastNotification(notification: NotificationObject) {
    let toastDiv = CreateToast(notification);
    document.getElementById("toastContainer")!.appendChild(toastDiv);
    let toast = new window.bootstrap.Toast(toastDiv, {});
    toast.show();
}

export function ShowToast(message: string) {
    const notification: NotificationObject = {
        uid: `manual_${Date.now()}`,
        type: "manual",
        message: message,
        dismissable: false
    };
    ShowToastNotification(notification);
}

export function ShowSavedToast() {
    const notification: NotificationObject = {
        uid: `saved_${Date.now()}`,
        type: "manual",
        message: "Setting Saved!",
        dismissable: false
    };
    ShowToastNotification(notification);
}