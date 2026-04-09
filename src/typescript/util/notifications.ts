import {HttpGetJson, HttpPostJson} from "./network";
import {GetPageRoute} from "./miscellaneous";
import {LogError} from "./log";
import {ShowToastNotification} from "../components/toast";
import "../../scss/components/notificationBell.scss";

export interface Notification {
    uid: string;
    type: string;
    message: string;
    dismissable: boolean;
}

let lastKnownCount = 0;

export async function ShowNotifications() {
    let notificationsResponse = await HttpGetJson("/notification");
    if (notificationsResponse[0] !== 200) {
        LogError("Could not fetch notifications");
        return;
    }
    const notifications: Notification[] = notificationsResponse[1].notifications || [];
    switch (GetPageRoute()) {
        case "":
            for (const notification of notifications) {
                if (notification.type === "system") {
                    ShowToastNotification(notification);
                }
            }
            break;
        case "p":
            for (const notification of notifications) {
                if (notification.type === "system") {
                    ShowToastNotification(notification);
                }
            }
            break;
    }
    initNotificationBell();
}
export async function DismissNotification(uid: string) {
    let csrfToken = (document.getElementById("csrfToken")! as HTMLInputElement).value;
    let response = await HttpPostJson(`/notification/dismiss/${uid}`, {}, csrfToken);
    if (response[0] !== 200) {
        LogError("Could not dismiss notification: " + uid);
    }
}
function initNotificationBell() {
    let isCookieAuthenticated = document.getElementById("isCookieAuthenticated") as HTMLInputElement | null;
    if (!isCookieAuthenticated || isCookieAuthenticated.value !== "true") return;
    let bell = document.getElementById("notificationBell");
    if (!bell) return;
    bell.style.display = "flex";
    updateBellBadge();
    setInterval(updateBellBadge, 60000);
    requestBrowserNotificationPermission();
}
async function updateBellBadge() {
    let response = await HttpGetJson("/notifications/count");
    if (response[0] !== 200) return;
    let count = response[1].count || 0;
    let badge = document.getElementById("notificationBadge");
    if (!badge) return;
    if (count > 0) {
        badge.textContent = count > 99 ? "99+" : String(count);
        badge.style.display = "block";
    } else {
        badge.style.display = "none";
    }
    if (count > lastKnownCount && lastKnownCount > 0) {
        let newCount = count - lastKnownCount;
        showBrowserNotification(newCount);
    }
    lastKnownCount = count;
}
function requestBrowserNotificationPermission() {
    if (!("Notification" in window)) return;
    if (Notification.permission === "granted") return;
    if (Notification.permission === "denied") return;
    if (localStorage.getItem("notificationPromptDismissed") === "true") return;
    Notification.requestPermission().then((permission) => {
        if (permission === "denied" || permission === "default") {
            localStorage.setItem("notificationPromptDismissed", "true");
        }
    });
}
function showBrowserNotification(count: number) {
    if (!("Notification" in window)) return;
    if (Notification.permission !== "granted") return;
    let body = count === 1 ? "You have a new notification" : `You have ${count} new notifications`;
    let notification = new Notification("YourPlace", {
        body: body,
        icon: "/static/image/yourplace-logo.svg",
    });
    notification.onclick = () => {
        window.focus();
        window.location.href = "/notifications";
        notification.close();
    };
}
