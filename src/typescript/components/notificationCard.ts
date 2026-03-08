import "../../scss/components/notificationCard.scss";
import {XSSSanitizeValue} from "../util/security";
import {WalletGetCachedName} from "../util/blockchain/wallet";

export interface UserNotification {
    id: string;
    fromAddress: string;
    fromBlockchain: string;
    reactionType: string;
    targetTxHash: string;
    timestamp: string;
    type: string;
}

export function CreateNotificationCard(notification: UserNotification, onDismiss: (id: string) => void): HTMLDivElement {
    let card = document.createElement("div");
    card.className = "notificationCard";
    card.setAttribute("data-notification-id", notification.id);
    let iconDiv = document.createElement("div");
    iconDiv.className = "notificationCardIcon";
    iconDiv.textContent = getNotificationIcon(notification);
    let bodyDiv = document.createElement("div");
    bodyDiv.className = "notificationCardBody";
    let messageDiv = document.createElement("div");
    messageDiv.className = "notificationCardMessage";
    let addressSpan = document.createElement("a");
    addressSpan.href = `/p/${XSSSanitizeValue(notification.fromBlockchain)}/${XSSSanitizeValue(notification.fromAddress)}`;
    addressSpan.textContent = truncateAddress(notification.fromAddress);
    addressSpan.className = "notificationCardAddress";
    resolveDisplayName(addressSpan, notification.fromBlockchain, notification.fromAddress);
    messageDiv.appendChild(addressSpan);
    messageDiv.appendChild(document.createTextNode(" " + getNotificationText(notification)));
    if (notification.targetTxHash) {
        let postLink = document.createElement("a");
        postLink.href = `/post/${XSSSanitizeValue(notification.fromBlockchain)}/${XSSSanitizeValue(notification.targetTxHash)}`;
        postLink.textContent = "View post";
        postLink.style.marginLeft = "0.3em";
        messageDiv.appendChild(postLink);
    }
    let timeDiv = document.createElement("div");
    timeDiv.className = "notificationCardTime";
    timeDiv.textContent = formatRelativeTime(parseInt(notification.timestamp));
    bodyDiv.appendChild(messageDiv);
    bodyDiv.appendChild(timeDiv);
    let dismissBtn = document.createElement("button");
    dismissBtn.className = "notificationCardDismiss";
    dismissBtn.title = "Dismiss";
    let dismissIcon = document.createElement("i");
    dismissIcon.className = "bi bi-x-lg";
    dismissBtn.appendChild(dismissIcon);
    dismissBtn.addEventListener("click", (e) => {
        e.preventDefault();
        e.stopPropagation();
        onDismiss(notification.id);
        card.remove();
    });
    card.appendChild(iconDiv);
    card.appendChild(bodyDiv);
    card.appendChild(dismissBtn);
    return card;
}

function formatRelativeTime(timestamp: number): string {
    let now = Math.floor(Date.now() / 1000);
    let diff = now - timestamp;
    if (diff < 60) return "just now";
    if (diff < 3600) return Math.floor(diff / 60) + "m ago";
    if (diff < 86400) return Math.floor(diff / 3600) + "h ago";
    return Math.floor(diff / 86400) + "d ago";
}
function getNotificationIcon(notification: UserNotification): string {
    switch (notification.type) {
        case "comment": return "💬";
        case "follow": return "👤";
        case "reaction":
            if (notification.reactionType === "like") return "👍";
            if (notification.reactionType === "dislike") return "👎";
            return notification.reactionType || "⭐";
        case "repost": return "🔁";
        default: return "🔔";
    }
}
function getNotificationText(notification: UserNotification): string {
    switch (notification.type) {
        case "comment": return "commented on your post.";
        case "follow": return "followed you.";
        case "reaction":
            if (notification.reactionType === "like") return "liked your post.";
            if (notification.reactionType === "dislike") return "disliked your post.";
            return "reacted to your post.";
        case "repost": return "reposted your post.";
        default: return "interacted with you.";
    }
}
function resolveDisplayName(element: HTMLAnchorElement, blockchain: string, address: string) {
    let name = WalletGetCachedName(blockchain, address);
    if (name) {
        element.textContent = name;
    }
}
function truncateAddress(address: string): string {
    if (address.length > 12) {
        return address.substring(0, 6) + "..." + address.substring(address.length - 4);
    }
    return address;
}
