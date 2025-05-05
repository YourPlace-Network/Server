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
const fileIcons: Record<string, string> = {
    ".pdf": "bi-file-earmark-pdf-fill",
    ".doc": "bi-file-earmark-word-fill",
    ".docx": "bi-file-earmark-word-fill",
    ".png": "bi-file-earmark-image-fill",
    ".jpg": "bi-file-earmark-image-fill",
    "jpeg": "bi-file-earmark-image-fill",
    ".webp": "bi-file-earmark-image-fill",
    ".mp4": "bi-file-earmark-play-fill",
    ".mpeg": "bi-file-earmark-play-fill",
    ".mov": "bi-file-earmark-play-fill",
    ".mkv": "bi-file-earmark-play-fill"
}
export function getFileIcon(extension: string): string {
    const lowerExtension = extension.toLowerCase();
    if (!fileIcons [lowerExtension]) {
        return "bi-file-earmark-fill";
    }
    return fileIcons[lowerExtension];
}