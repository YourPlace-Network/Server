import {HttpGetJson, HttpPostJson} from "../util/network";
import {LogError, LogInfo} from "../util/log";

export async function XcomIsCrossPostEnabled(): Promise<boolean> {
    try {
        const response = await HttpGetJson("/settings/services/xcom/crosspost");
        if (response[0] !== 200) {
            return false;
        }
        return response[1].enabled === true;
    } catch (error: any) {
        LogError("X.com CrossPost Check Error: " + error);
        return false;
    }
}
export async function XcomCrossPost(text: string, csrfToken: string): Promise<boolean> {
    try {
        const response = await HttpPostJson("/services/xcom/post", {text: text}, csrfToken);
        if (response[0] !== 200) {
            LogError("X.com CrossPost Error: " + response[1].status);
            return false;
        }
        LogInfo("X.com CrossPost Success");
        return true;
    } catch (error: any) {
        LogError("X.com CrossPost Error: " + error);
        return false;
    }
}

export interface App {
    id: string;
    name: string;
    description: string;
}
export interface Account {
    id: string;
    apps: App[];
    email: string;
}
export interface FeedItem {
    id: string;
    content: string;
    timestamp: string;
    userId: string;
}

export class ProfileService {
    private accountId = "";

    constructor(accountId: string) {
        this.accountId = accountId;
    }

    public async attachApp(appId: string): Promise<Account | null> {
        let csrfToken = (document.getElementById("csrfToken")! as HTMLInputElement).value;
        try {
            const response = await HttpPostJson("/service/twitter/addApp", {
               accountId: this.accountId,
                appId: appId
            }, csrfToken);
            if (response[0] !== 200) {
                LogError("Twitter Attach Error: " + response[1]);
                return null;
            }
            return response[1] as Account;
        } catch (error: any) {
            LogError("Twitter Attach Error: " + error);
            return null;
        }
    }
    public async getProfileFeed(): Promise<FeedItem[] | null> {
        try {
            const response = await HttpGetJson("/service/twitter/profileFeed/");
            if (response[0] !== 200) {
                LogError("Twitter Feed Error: " + response[1]);
                return null;
            }
            return response[1] as FeedItem[];
        } catch (error: any) {
            LogError("Twitter Feed Error: " + error);
            return null;
        }
    }
    public async getAvailableApps(): Promise<App[] | null> {
        try {
            const response = await HttpGetJson("/service/twitter/availableApps");
            if (response[0] !== 200) {
                LogError("Twitter Apps Error: " + response[1]);
                return null;
            }
            return response[1] as App[];
        } catch (error: any) {
            LogError("Twitter Apps Error: " + error);
            return null;
        }
    }
}