import { GetAddress, GetChain, WalletSubmitDislike, WalletSubmitEmojiReaction, WalletSubmitLike } from "../util/blockchain/wallet";
import { HttpGetJson } from "../util/network";
import { ShowDialogModal } from "./modalDialog";
import { createEmojiPicker, closeEmojiPicker } from "../util/emojiPicker";

export interface PostControlsOptions {
    blockchain: string;
    initialComments?: number;
    initialDislikes?: number;
    initialEmojiCount?: number;
    initialLikes?: number;
    onCommentClick?: () => void;
    onRepostClick?: () => void;
    targetType: 'post' | 'comment';
    txHash: string;
    userEmojiReaction?: string | null;
    userHasCommented?: boolean;
    userReaction?: string | null;
}
export interface ReactionCounts {
    likes: number;
    dislikes: number;
    emoji: { [key: string]: number };
    userReaction?: string | null;
}
export interface UserReactions {
    reaction: string | null;
    emojiReaction: string | null;
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
    if (options.userHasCommented) {
        commentControl.classList.add("active");
    }
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
    const reactControl = createReactControlItem(options.initialEmojiCount || 0, options.userEmojiReaction || null, (e) => {
        showReactionsPopup(e.currentTarget as HTMLElement, options.txHash, options.blockchain, options.targetType);
    });
    reactControl.classList.add("react");
    if (options.userEmojiReaction) {
        reactControl.classList.add("active");
    }
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
function createReactControlItem(count: number, userEmoji: string | null, onClick: (e: MouseEvent) => void): HTMLDivElement {
    const item = document.createElement("div");
    item.classList.add("postControlItem");
    item.title = "React";
    if (userEmoji) {
        const emojiSpan = document.createElement("span");
        emojiSpan.classList.add("reactEmoji");
        emojiSpan.textContent = userEmoji;
        item.appendChild(emojiSpan);
    } else {
        const icon = document.createElement("i");
        icon.classList.add("bi", "bi-emoji-smile");
        item.appendChild(icon);
    }
    const countSpan = document.createElement("span");
    countSpan.classList.add("count");
    countSpan.textContent = count > 0 ? count.toString() : "";
    item.appendChild(countSpan);
    item.addEventListener("click", onClick);
    return item;
}
function updateReactControlIcon(reactControl: Element, emoji: string): void {
    const existingIcon = reactControl.querySelector("i.bi");
    const existingEmoji = reactControl.querySelector(".reactEmoji");
    if (existingIcon) {
        existingIcon.remove();
    }
    if (existingEmoji) {
        existingEmoji.textContent = emoji;
    } else {
        const emojiSpan = document.createElement("span");
        emojiSpan.classList.add("reactEmoji");
        emojiSpan.textContent = emoji;
        reactControl.insertBefore(emojiSpan, reactControl.firstChild);
    }
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
export async function FetchUserHasCommented(blockchain: string, txHash: string, address: string): Promise<boolean> {
    try {
        const response = await HttpGetJson(`/comments/${blockchain}/${txHash}/user/${address}`);
        if (response[0] === 200 && response[1]) {
            return response[1].hasCommented === true;
        }
    } catch (e) {
        console.error("Failed to fetch user comment status:", e);
    }
    return false;
}
export async function FetchUserReaction(blockchain: string, txHash: string, address: string): Promise<UserReactions | null> {
    try {
        const response = await HttpGetJson(`/reactions/${blockchain}/${txHash}/user/${address}`);
        if (response[0] === 200 && response[1]) {
            return {
                reaction: response[1].reaction || null,
                emojiReaction: response[1].emojiReaction || null
            };
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
    addReactionBtn.classList.add("addReactionBtn");
    addReactionBtn.innerHTML = '<i class="bi bi-plus-circle"></i> Add Reaction';
    popup.appendChild(addReactionBtn);
    const pickerContainer = document.createElement("div");
    pickerContainer.classList.add("pickerContainer");
    pickerContainer.style.display = "none";
    popup.appendChild(pickerContainer);
    loadExistingReactions(existingReactionsDiv, txHash, blockchain, targetType);
    addReactionBtn.addEventListener("click", () => {
        if (pickerContainer.style.display === "none") {
            const picker = createEmojiPicker(async (emoji: string) => {
                const address = GetAddress();
                if (!address) {
                    ShowDialogModal("Please connect your wallet to react");
                    return;
                }
                await WalletSubmitEmojiReaction(txHash, targetType, emoji);
                const controlsBar = document.querySelector(`.postControlsBar[data-txhash="${txHash}"][data-blockchain="${blockchain}"]`);
                if (controlsBar) {
                    const reactControl = controlsBar.querySelector(".react");
                    if (reactControl) {
                        reactControl.classList.add("active");
                        updateReactControlIcon(reactControl, emoji);
                    }
                }
                if (activeReactionsPopup) {
                    activeReactionsPopup.remove();
                    activeReactionsPopup = null;
                }
            });
            pickerContainer.appendChild(picker);
            pickerContainer.style.display = "block";
            addReactionBtn.innerHTML = '<i class="bi bi-dash-circle"></i> Hide Picker';
        } else {
            pickerContainer.innerHTML = "";
            pickerContainer.style.display = "none";
            addReactionBtn.innerHTML = '<i class="bi bi-plus-circle"></i> Add Reaction';
        }
    });
    const rect = targetElement.getBoundingClientRect();
    const scrollX = window.scrollX || window.pageXOffset;
    const scrollY = window.scrollY || window.pageYOffset;
    popup.style.position = "absolute";
    popup.style.left = `${rect.left + scrollX}px`;
    popup.style.top = `${rect.bottom + scrollY + 5}px`;
    document.body.appendChild(popup);
    activeReactionsPopup = popup;
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
    let userEmojiReaction: string | null = null;
    if (address) {
        const userReactions = await FetchUserReaction(blockchain, txHash, address);
        if (userReactions) {
            userEmojiReaction = userReactions.emojiReaction;
        }
    }
    container.innerHTML = "";
    for (const [emoji, count] of Object.entries(counts.emoji)) {
        if (count > 0) {
            const chip = document.createElement("div");
            chip.classList.add("reactionChip");
            if (userEmojiReaction === emoji) {
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
                const controlsBar = document.querySelector(`.postControlsBar[data-txhash="${txHash}"][data-blockchain="${blockchain}"]`);
                if (controlsBar) {
                    const reactControl = controlsBar.querySelector(".react");
                    if (reactControl) {
                        reactControl.classList.add("active");
                        updateReactControlIcon(reactControl, emoji);
                    }
                }
            });
            container.appendChild(chip);
        }
    }
}
