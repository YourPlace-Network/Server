import {HttpPostJson} from "./network";
import {GetPageRoute} from "./miscellaneous";
import {LogError} from "./log";

export interface NotificationObject {
    uid: string;
    type: string;
    message: string;
    dismissable: boolean;
}

export async function GetNotifications() {
    let route = GetPageRoute();
    switch (route) {
        case "": // home page
            console.log("working on home");
            break;
        case "p": // profile page
            console.log("working on profile");
            break;
    }
}
export async function DismissNotification(uid: string) {
    let response = await HttpPostJson(`/notification/dismiss/${uid}`, {}, "");
    if (response[0] !== 200) {
        LogError("Could not dismiss notification: " + uid);
    }
}