import {LogError, LogInfo} from "../util/log";
import {HttpGetJson, HttpPostJson} from "../util/network";

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
