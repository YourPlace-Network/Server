import {HttpGetJson, HttpPostFile, HttpPostJson} from "./network";

export async function UploadFile(file: File | FileList, csrfToken: string): Promise<[number, any]> {
    if (csrfToken == null || csrfToken == "") {
        return [400, {"status": "Invalid CSRF Token"}];
    }
    let response = await HttpPostFile("/files/upload", file, csrfToken);
    return [response[0], response[1]];
}
export async function DownloadFile(cid: string): Promise<[number, any]> {
    let response = await HttpGetJson("/files/download/" + cid);
    return [response[0], response[1]];
}
export async function FinalizeFiles(cids: string[], visibility: "public" | "private", source: string, csrfToken: string, txHash?: string, blockchain?: string): Promise<[number, any]> {
    return await HttpPostJson("/files/finalize", {
        cids,
        visibility,
        source,
        txHash: txHash || "",
        blockchain: blockchain || "",
    }, csrfToken);
}
export async function DeleteFile(cid: string, csrfToken: string, txHash?: string, blockchain?: string): Promise<[number, any]> {
    return await HttpPostJson("/files/delete", {
        cid,
        txHash: txHash || "",
        blockchain: blockchain || "",
    }, csrfToken);
}
export async function PrepareRenameFile(cid: string, fileNameBase: string, csrfToken: string): Promise<[number, any]> {
    return await HttpPostJson("/files/rename/prepare", {
        cid,
        fileNameBase,
    }, csrfToken);
}
export async function RenameFile(cid: string, fileNameBase: string, csrfToken: string, publishCid?: string, deleteTxHash?: string, publishTxHash?: string, blockchain?: string): Promise<[number, any]> {
    return await HttpPostJson("/files/rename", {
        cid,
        fileNameBase,
        publishCid: publishCid || "",
        deleteTxHash: deleteTxHash || "",
        publishTxHash: publishTxHash || "",
        blockchain: blockchain || "",
    }, csrfToken);
}
export async function CreateLocalPost(payload: string, attachments: string[], csrfToken: string): Promise<[number, any]> {
    return await HttpPostJson("/posts/local", {payload, attachments}, csrfToken);
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
