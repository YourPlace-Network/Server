import { GetAddress, GetChain, WalletSubmitDislike, WalletSubmitEmojiReaction, WalletSubmitLike } from "../util/blockchain/wallet";
import { HttpGetJson } from "../util/network";
import { ShowDialogModal } from "./modalDialog";

export interface PostControlsOptions {
    txHash: string;
    blockchain: string;
    targetType: 'post' | 'comment';
    initialLikes?: number;
    initialDislikes?: number;
    initialComments?: number;
    userReaction?: string | null;
    onCommentClick?: () => void;
    onRepostClick?: () => void;
}
export interface ReactionCounts {
    likes: number;
    dislikes: number;
    emoji: { [key: string]: number };
    userReaction?: string | null;
}
export function CreatePostControlsBar(options: PostControlsOptions): HTMLDivElement {
    const controlsBar = document.createElement("div");
    controlsBar.classList.add("postControlsBar");
    controlsBar.dataset.txhash = options.txHash;
    controlsBar.dataset.blockchain = options.blockchain;
    controlsBar.dataset.targettype = options.targetType;
    const commentControl = createControlItem("bi-chat", options.initialComments || 0, "Comment", () => {
        if (options.onCommentClick) {
            options.onCommentClick();
        }
    });
    commentControl.classList.add("comment");
    const likeControl = createControlItem("bi-hand-thumbs-up", options.initialLikes || 0, "Like", async () => {
        const address = GetAddress();
        if (!address) {
            ShowDialogModal("Please connect your wallet to like posts");
            return;
        }
        await WalletSubmitLike(options.txHash, options.targetType);
        likeControl.classList.add("active");
        dislikeControl.classList.remove("active");
        updateCount(likeControl, (options.initialLikes || 0) + 1);
    });
    likeControl.classList.add("like");
    if (options.userReaction === "like") {
        likeControl.classList.add("active");
    }
    const dislikeControl = createControlItem("bi-hand-thumbs-down", options.initialDislikes || 0, "Dislike", async () => {
        const address = GetAddress();
        if (!address) {
            ShowDialogModal("Please connect your wallet to dislike posts");
            return;
        }
        await WalletSubmitDislike(options.txHash, options.targetType);
        dislikeControl.classList.add("active");
        likeControl.classList.remove("active");
        updateCount(dislikeControl, (options.initialDislikes || 0) + 1);
    });
    dislikeControl.classList.add("dislike");
    if (options.userReaction === "dislike") {
        dislikeControl.classList.add("active");
    }
    const reactControl = createControlItem("bi-emoji-smile", 0, "React", (e) => {
        showReactionsPopup(e.currentTarget as HTMLElement, options.txHash, options.blockchain, options.targetType);
    });
    reactControl.classList.add("react");
    const repostControl = createControlItem("bi-arrow-repeat", 0, "Repost", () => {
        if (options.onRepostClick) {
            options.onRepostClick();
        }
    });
    repostControl.classList.add("repost");
    controlsBar.appendChild(commentControl);
    controlsBar.appendChild(likeControl);
    controlsBar.appendChild(dislikeControl);
    controlsBar.appendChild(reactControl);
    controlsBar.appendChild(repostControl);
    return controlsBar;
}
function createControlItem(iconClass: string, count: number, tooltip: string, onClick: (e: MouseEvent) => void): HTMLDivElement {
    const item = document.createElement("div");
    item.classList.add("postControlItem");
    item.title = tooltip;
    const icon = document.createElement("i");
    icon.classList.add("bi", iconClass);
    item.appendChild(icon);
    const countSpan = document.createElement("span");
    countSpan.classList.add("count");
    countSpan.textContent = count > 0 ? count.toString() : "";
    item.appendChild(countSpan);
    item.addEventListener("click", onClick);
    return item;
}
function updateCount(element: HTMLElement, newCount: number): void {
    const countSpan = element.querySelector(".count");
    if (countSpan) {
        countSpan.textContent = newCount > 0 ? newCount.toString() : "";
    }
}
export function UpdateReactionCounts(controlsBar: HTMLDivElement, counts: ReactionCounts): void {
    const likeControl = controlsBar.querySelector(".postControlItem.like");
    const dislikeControl = controlsBar.querySelector(".postControlItem.dislike");
    if (likeControl) {
        updateCount(likeControl as HTMLElement, counts.likes);
        if (counts.userReaction === "like") {
            likeControl.classList.add("active");
        } else {
            likeControl.classList.remove("active");
        }
    }
    if (dislikeControl) {
        updateCount(dislikeControl as HTMLElement, counts.dislikes);
        if (counts.userReaction === "dislike") {
            dislikeControl.classList.add("active");
        } else {
            dislikeControl.classList.remove("active");
        }
    }
}
export async function FetchReactionCounts(blockchain: string, txHash: string): Promise<ReactionCounts | null> {
    try {
        const response = await HttpGetJson(`/reactions/${blockchain}/${txHash}`);
        if (response[0] === 200 && response[1]) {
            return response[1] as ReactionCounts;
        }
    } catch (e) {
        console.error("Failed to fetch reaction counts:", e);
    }
    return null;
}
export async function FetchUserReaction(blockchain: string, txHash: string, address: string): Promise<string | null> {
    try {
        const response = await HttpGetJson(`/reactions/${blockchain}/${txHash}/user/${address}`);
        if (response[0] === 200 && response[1]) {
            return response[1].reaction || null;
        }
    } catch (e) {
        console.error("Failed to fetch user reaction:", e);
    }
    return null;
}
let activeReactionsPopup: HTMLElement | null = null;
function showReactionsPopup(targetElement: HTMLElement, txHash: string, blockchain: string, targetType: string): void {
    if (activeReactionsPopup) {
        activeReactionsPopup.remove();
        activeReactionsPopup = null;
    }
    const popup = document.createElement("div");
    popup.classList.add("reactionsPopup");
    const existingReactionsDiv = document.createElement("div");
    existingReactionsDiv.classList.add("existingReactions");
    popup.appendChild(existingReactionsDiv);
    const addReactionBtn = document.createElement("button");
    addReactionBtn.classList.add("btn", "btn-sm", "btn-outline-secondary");
    addReactionBtn.textContent = "Add Reaction";
    addReactionBtn.addEventListener("click", () => {
        showEmojiPicker(popup, txHash, blockchain, targetType);
    });
    popup.appendChild(addReactionBtn);
    const rect = targetElement.getBoundingClientRect();
    popup.style.position = "fixed";
    popup.style.left = `${rect.left}px`;
    popup.style.top = `${rect.bottom + 5}px`;
    document.body.appendChild(popup);
    activeReactionsPopup = popup;
    loadExistingReactions(existingReactionsDiv, txHash, blockchain, targetType);
    const closeOnOutsideClick = (e: MouseEvent) => {
        if (!popup.contains(e.target as Node) && e.target !== targetElement) {
            popup.remove();
            activeReactionsPopup = null;
            document.removeEventListener("click", closeOnOutsideClick);
        }
    };
    setTimeout(() => {
        document.addEventListener("click", closeOnOutsideClick);
    }, 100);
}
async function loadExistingReactions(container: HTMLElement, txHash: string, blockchain: string, targetType: string): Promise<void> {
    const counts = await FetchReactionCounts(blockchain, txHash);
    if (!counts || !counts.emoji) return;
    const address = GetAddress();
    let userReaction: string | null = null;
    if (address) {
        userReaction = await FetchUserReaction(blockchain, txHash, address);
    }
    container.innerHTML = "";
    for (const [emoji, count] of Object.entries(counts.emoji)) {
        if (count > 0) {
            const chip = document.createElement("div");
            chip.classList.add("reactionChip");
            if (userReaction === emoji) {
                chip.classList.add("selected");
            }
            const emojiSpan = document.createElement("span");
            emojiSpan.classList.add("emoji");
            emojiSpan.textContent = emoji;
            chip.appendChild(emojiSpan);
            const countSpan = document.createElement("span");
            countSpan.classList.add("count");
            countSpan.textContent = count.toString();
            chip.appendChild(countSpan);
            chip.addEventListener("click", async () => {
                const addr = GetAddress();
                if (!addr) {
                    ShowDialogModal("Please connect your wallet to react");
                    return;
                }
                await WalletSubmitEmojiReaction(txHash, targetType, emoji);
                chip.classList.add("selected");
            });
            container.appendChild(chip);
        }
    }
}
const commonEmojis = ["👍", "👎", "😀", "😂", "😍", "😢", "😮", "🎉", "❤️", "🔥", "👏", "🙏", "💯", "✨", "🚀", "💪"];
function showEmojiPicker(popup: HTMLElement, txHash: string, blockchain: string, targetType: string): void {
    let existingPicker = popup.querySelector(".emojiGrid");
    if (existingPicker) {
        existingPicker.remove();
        return;
    }
    const grid = document.createElement("div");
    grid.classList.add("emojiGrid");
    for (const emoji of commonEmojis) {
        const btn = document.createElement("button");
        btn.classList.add("emojiButton");
        btn.textContent = emoji;
        btn.addEventListener("click", async () => {
            const address = GetAddress();
            if (!address) {
                ShowDialogModal("Please connect your wallet to react");
                return;
            }
            await WalletSubmitEmojiReaction(txHash, targetType, emoji);
            if (activeReactionsPopup) {
                activeReactionsPopup.remove();
                activeReactionsPopup = null;
            }
        });
        grid.appendChild(btn);
    }
    popup.appendChild(grid);
}
