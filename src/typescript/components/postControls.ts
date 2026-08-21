import { GetAddress, WalletSubmitDislike, WalletSubmitEmojiReaction, WalletSubmitLike } from "../util/blockchain/wallet";
import { HttpGetJson } from "../util/network";
import { createEmojiPicker } from "../util/emojiPicker";
import { setRedirect, useRedirect } from "../util/redirect";

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
    dislikes: number;
    emoji: { [key: string]: number };
    likes: number;
    userEmojiReaction?: string | null;
    userReaction?: string | null;
}

const CONTROLS_REFRESH_INTERVAL_MS = 30000;
const refreshingControlsBars = new Set<HTMLDivElement>();
let controlsRefreshTimer: number | null = null;

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
            await handleAuthRedirect("like", {targetTxHash: options.txHash, targetType: options.targetType}, controlsBar);
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
            await handleAuthRedirect("dislike", {targetTxHash: options.txHash, targetType: options.targetType}, controlsBar);
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
    StartPostControlsRefresh(controlsBar);
    return controlsBar;
}
async function handleAuthRedirect(action: string, variable: Record<string, unknown>, controlsBar?: HTMLDivElement | null): Promise<boolean> {
    if (!setRedirect(action, {...variable, path: getCurrentRelativePath()})) {
        window.location.href = "/login";
        return false;
    }
    const usedRedirect = await useRedirect();
    if (!usedRedirect) {
        window.location.href = "/login";
        return false;
    }
    if (controlsBar) {
        await RefreshPostControlsBar(controlsBar);
    }
    return true;
}
function getCurrentRelativePath(): string {
    return window.location.pathname + window.location.search + window.location.hash;
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
function clearReactControlIcon(reactControl: Element): void {
    const existingEmoji = reactControl.querySelector(".reactEmoji");
    if (existingEmoji) {
        existingEmoji.remove();
    }
    if (!reactControl.querySelector("i.bi")) {
        const icon = document.createElement("i");
        icon.classList.add("bi", "bi-emoji-smile");
        reactControl.insertBefore(icon, reactControl.firstChild);
    }
}
function getTotalEmojiCount(counts: ReactionCounts): number {
    if (!counts || !counts.emoji) return 0;
    let total = 0;
    for (const c of Object.values(counts.emoji)) {
        total += c;
    }
    return total;
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
    const reactControl = controlsBar.querySelector(".postControlItem.react");
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
    if (reactControl) {
        const userEmojiReaction = counts.userEmojiReaction || null;
        updateCount(reactControl as HTMLElement, getTotalEmojiCount(counts));
        if (userEmojiReaction) {
            reactControl.classList.add("active");
            updateReactControlIcon(reactControl, userEmojiReaction);
        } else {
            reactControl.classList.remove("active");
            clearReactControlIcon(reactControl);
        }
    }
}
export async function FetchReactionCounts(blockchain: string, txHash: string, address?: string): Promise<ReactionCounts | null> {
    try {
        let url = `/reactions/${blockchain}/${txHash}`;
        if (address) {
            url += `?address=${encodeURIComponent(address)}`;
        }
        const response = await HttpGetJson(url);
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
export async function FetchCommentCount(blockchain: string, txHash: string): Promise<number | null> {
    try {
        const response = await HttpGetJson(`/comments/${blockchain}/${txHash}/count`);
        if (response[0] === 200 && response[1]) {
            return Number(response[1].count || 0);
        }
    } catch (e) {
        console.error("Failed to fetch comment count:", e);
    }
    return null;
}
async function RefreshPostControlsBar(controlsBar: HTMLDivElement): Promise<void> {
    const blockchain = controlsBar.dataset.blockchain;
    const txHash = controlsBar.dataset.txhash;
    if (!blockchain || !txHash) return;
    const address = GetAddress();
    const [counts, commentCount, hasCommented] = await Promise.all([
        FetchReactionCounts(blockchain, txHash, address || undefined),
        FetchCommentCount(blockchain, txHash),
        address ? FetchUserHasCommented(blockchain, txHash, address) : Promise.resolve(false),
    ]);
    if (counts) {
        UpdateReactionCounts(controlsBar, counts);
    }
    const commentControl = controlsBar.querySelector(".postControlItem.comment");
    if (commentControl) {
        if (commentCount !== null) {
            updateCount(commentControl as HTMLElement, commentCount);
        }
        if (hasCommented) {
            commentControl.classList.add("active");
        } else {
            commentControl.classList.remove("active");
        }
    }
}
function RefreshRegisteredControlsBars(): void {
    refreshingControlsBars.forEach((controlsBar) => {
        if (!document.body.contains(controlsBar)) {
            refreshingControlsBars.delete(controlsBar);
            return;
        }
        RefreshPostControlsBar(controlsBar).catch(e => console.error("Failed to refresh post controls:", e));
    });
    if (refreshingControlsBars.size === 0 && controlsRefreshTimer !== null) {
        window.clearInterval(controlsRefreshTimer);
        controlsRefreshTimer = null;
    }
}
export function StartPostControlsRefresh(controlsBar: HTMLDivElement): void {
    refreshingControlsBars.add(controlsBar);
    window.setTimeout(() => {
        if (document.body.contains(controlsBar)) {
            RefreshPostControlsBar(controlsBar).catch(e => console.error("Failed to refresh post controls:", e));
        }
    }, 0);
    if (controlsRefreshTimer === null) {
        controlsRefreshTimer = window.setInterval(RefreshRegisteredControlsBars, CONTROLS_REFRESH_INTERVAL_MS);
    }
}

let activeReactionsPopup: HTMLElement | null = null;
function closeActiveReactionsPopup(): void {
    if (activeReactionsPopup) {
        activeReactionsPopup.remove();
        activeReactionsPopup = null;
    }
}
function showReactionsPopup(targetElement: HTMLElement, txHash: string, blockchain: string, targetType: string): void {
    if (activeReactionsPopup) {
        const wasForSameTarget = activeReactionsPopup.dataset.txhash === txHash;
        activeReactionsPopup.remove();
        activeReactionsPopup = null;
        if (wasForSameTarget) return;
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
    const targetControlsBar = targetElement.closest(".postControlsBar") as HTMLDivElement | null;
    loadExistingReactions(existingReactionsDiv, txHash, blockchain, targetType, targetControlsBar);
    addReactionBtn.addEventListener("click", () => {
        if (pickerContainer.style.display === "none") {
            const picker = createEmojiPicker(async (emoji: string) => {
                const address = GetAddress();
                if (!address) {
                    if (await handleAuthRedirect("reaction", {targetTxHash: txHash, targetType, emoji}, targetControlsBar)) {
                        closeActiveReactionsPopup();
                    }
                    return;
                }
                await WalletSubmitEmojiReaction(txHash, targetType, emoji);
                const controlsBar = targetControlsBar || document.querySelector(`.postControlsBar[data-txhash="${txHash}"][data-blockchain="${blockchain}"]`);
                if (controlsBar) {
                    const reactControl = controlsBar.querySelector(".react");
                    if (reactControl) {
                        const hadReaction = reactControl.classList.contains("active");
                        reactControl.classList.add("active");
                        updateReactControlIcon(reactControl, emoji);
                        if (!hadReaction) {
                            const currentCount = parseInt(reactControl.querySelector(".count")?.textContent || "0", 10);
                            updateCount(reactControl as HTMLElement, currentCount + 1);
                        }
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
    popup.dataset.txhash = txHash;
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
async function loadExistingReactions(container: HTMLElement, txHash: string, blockchain: string, targetType: string, targetControlsBar: HTMLDivElement | null): Promise<void> {
    const address = GetAddress();
    const counts = await FetchReactionCounts(blockchain, txHash, address || undefined);
    if (!counts || !counts.emoji) return;
    const userEmojiReaction = counts.userEmojiReaction || null;
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
                    if (await handleAuthRedirect("reaction", {targetTxHash: txHash, targetType, emoji}, targetControlsBar)) {
                        closeActiveReactionsPopup();
                    }
                    return;
                }
                await WalletSubmitEmojiReaction(txHash, targetType, emoji);
                container.querySelectorAll(".reactionChip.selected").forEach(el => el.classList.remove("selected"));
                chip.classList.add("selected");
                const controlsBar = targetControlsBar || document.querySelector(`.postControlsBar[data-txhash="${txHash}"][data-blockchain="${blockchain}"]`);
                if (controlsBar) {
                    const reactControl = controlsBar.querySelector(".react");
                    if (reactControl) {
                        const hadReaction = reactControl.classList.contains("active");
                        reactControl.classList.add("active");
                        updateReactControlIcon(reactControl, emoji);
                        const total = getTotalEmojiCount(counts);
                        updateCount(reactControl as HTMLElement, hadReaction ? total : total + 1);
                    }
                }
            });
            container.appendChild(chip);
        }
    }
}
