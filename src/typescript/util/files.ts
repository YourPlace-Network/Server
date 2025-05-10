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
    "application/pdf": "bi-file-earmark-pdf-fill",
    "application/msword": "bi-file-earmark-word-fill",
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document": "bi-file-earmark-word-fill",
    "image/png": "bi-file-earmark-image-fill",
    "image/jpeg": "bi-file-earmark-image-fill",
    "image/webp": "bi-file-earmark-image-fill",
    "video/mp4": "bi-file-earmark-play-fill",
    "video/mpeg": "bi-file-earmark-play-fill",
    "video/quicktime": "bi-file-earmark-play-fill",
    "video/x-matroshka": "bi-file-earmark-play-fill"
}
export function getFileIcon(mimeType: string): string {
    const lowerExtension = mimeType.toLowerCase();
    if (!fileIcons [lowerExtension]) {
        return "bi-file-earmark-fill";
    }
    return fileIcons[lowerExtension];
}
export async function formatFileSize(bytes: number): Promise<string> {
    if (bytes === 0) return '0 Bytes';
    const units = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    const unitIndex = Math.min(i, units.length - 1);
    const size = bytes / Math.pow(1024, unitIndex);
    return `${size.toFixed(2)} ${units[unitIndex]}`;
}