import {HttpPostJson, HttpGetJson} from "../util/network";

export async function AIIsEnabled(): Promise<boolean> {
    let response = await HttpGetJson("/service/ai/ollamaEnabled");
    return response[0] === 200 && response[1].status === "enabled";
}
export async function AIIsModelEnabled(): Promise<boolean> {
    let response = await HttpGetJson("/service/ai/ollamaModelEnabled");
    return response[0] === 200 && response[1].status === "enabled";
}
export async function AIGetSpiciness(quote: string): Promise<number> {
    let csrfToken = (document.getElementById("csrfToken")! as HTMLInputElement).value;
    let response = await HttpPostJson("/service/ai/spiciness/", {quote: quote}, csrfToken);
    if (response[0] != 200) {
        return -1;
    }
    return response[1].spiciness;
}