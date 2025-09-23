import {HttpGetJson, HttpPostJson} from "./network";
import {GetPageRoute} from "./miscellaneous";
import {LogError} from "./log";
import {ShowToastNotification} from "../components/toast";

export interface Notification {
    uid: string;
    type: string;
    message: string;
    dismissable: boolean;
}

export async function ShowNotifications() { // Main notification dispatcher
    let notificationsResponse = await HttpGetJson("/notification");
    if (notificationsResponse[0] !== 200) {
        LogError("Could not fetch notifications");
        return;
    }
    const notifications: Notification[] = notificationsResponse[1].notifications;
    switch (GetPageRoute()) {
        case "": // home page
            break;
        case "p": // profile page
            for (const notification of notifications) {
                if (notification.type === "system") {
                    ShowToastNotification(notification);
                }
            }
            break;
    }
}
export async function DismissNotification(uid: string) {
    let csrfToken = (document.getElementById("csrfToken")! as HTMLInputElement).value;
    let response = await HttpPostJson(`/notification/dismiss/${uid}`, {}, csrfToken);
    if (response[0] !== 200) {
        LogError("Could not dismiss notification: " + uid);
    }
}
