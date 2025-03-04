import {HttpPostJson} from "../util/network";

export async function uploadFile(file: File): Promise<string> {
    let formData = new FormData();
    let csrfToken = document.getElementById("csrfToken")! as HTMLInputElement
    formData.append("file", file);
    const [status, data] = await HttpPostJson("/files/upload", formData, csrfToken.value);
    return data["cid"];
}