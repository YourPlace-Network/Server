import "../../scss/components/fileCard.scss";
import { HttpGetJson } from "../util/network";
import { CreatePostCard } from "./postCard";

export type FileCardAttachment = [string, string, number | string, string, string?];
export interface FileCardData {
    address: string;
    author?: string;
    avatarSrc?: string;
    blockchain: string;
    localPost?: boolean;
    source: string;
    timestamp: number;
    txHash?: string;
    attachments: FileCardAttachment[];
}

async function fetchCommentCount(blockchain: string, txHash?: string): Promise<number> {
    if (!txHash) {
        return 0;
    }
    try {
        const response = await HttpGetJson(`/comments/${encodeURIComponent(blockchain)}/${encodeURIComponent(txHash)}/count`);
        if (response[0] !== 200 || !response[1]) {
            return 0;
        }
        return Number(response[1].count || 0);
    } catch (_) {
        return 0;
    }
}

export async function CreateFileCard(fileData: FileCardData): Promise<HTMLDivElement> {
    const commentCount = await fetchCommentCount(fileData.blockchain, fileData.txHash);
    return await CreatePostCard({
        resultType: "file",
        txHash: fileData.txHash || "",
        timestamp: fileData.timestamp,
        payload: "",
        blockchain: fileData.blockchain,
        address: fileData.address,
        author: fileData.author,
        avatarSrc: fileData.avatarSrc,
        localPost: fileData.localPost === true,
        source: fileData.source,
        attachments: fileData.attachments,
        commentCount: commentCount,
    });
}
