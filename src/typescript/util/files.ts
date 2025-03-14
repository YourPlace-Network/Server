import {HttpGetJson, HttpPostFile} from "./network";

export async function UploadFile(file: File | FileList, csrfToken: string): Promise<[number, any]> {
    if (csrfToken == null || csrfToken == "") {
        return [400, {"status": "Invalid CSRF Token"}];
    }
    let response = await HttpPostFile("/files/upload", file, csrfToken);
    return [response[0], response[1]];
}
export async function DownloadFile(uuid: string): Promise<[number, any]> {
    let response = await HttpGetJson("/files/download/" + uuid);
    return [response[0], response[1]];
}