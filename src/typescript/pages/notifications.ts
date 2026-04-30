window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/pages/notifications.scss";
import "../components/scrollTop";
import "../../scss/components/scrollTop.scss";
import "../components/menu";
import {CreateNotificationCard, type UserNotification} from "../components/notificationCard";
import {WalletGetAvatar, WalletGetCachedAvatar} from "../util/blockchain/wallet";
import {HttpGetJson, HttpPostJson} from "../util/network";
import {LogError} from "../util/log";
import {ShowNotifications} from "../util/notifications";

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        let DOM = {
            clearAllBtn: document.getElementById("clearAllBtn")! as HTMLButtonElement,
            csrfToken: (document.getElementById("csrfToken")! as HTMLInputElement).value,
            isCookieAuthenticated: document.getElementById("isCookieAuthenticated")! as HTMLInputElement,
            notificationsAvatar: document.getElementById("notificationsAvatar") as HTMLImageElement | null,
            notificationsDiv: document.getElementById("notificationsDiv")! as HTMLDivElement,
            notificationsEmpty: document.getElementById("notificationsEmpty")! as HTMLDivElement,
            userAddress: document.getElementById("userAddress")! as HTMLInputElement,
            userBlockchain: document.getElementById("userBlockchain")! as HTMLInputElement,
        }
        const PAGE_SIZE = 25;
        let offset = 0;
        let loading = false;
        let hasMore = true;

        async function renderNotificationsAvatar() {
            if (!DOM.notificationsAvatar || DOM.userAddress.value === "" || DOM.userBlockchain.value === "") {
                return;
            }
            const defaultAvatar = "/static/image/avatar.png";
            DOM.notificationsAvatar.onerror = () => {
                DOM.notificationsAvatar!.src = defaultAvatar;
                DOM.notificationsAvatar!.onerror = null;
            };
            const cachedAvatar = WalletGetCachedAvatar(DOM.userBlockchain.value, DOM.userAddress.value);
            DOM.notificationsAvatar.src = cachedAvatar || defaultAvatar;
            const avatarUrl = await WalletGetAvatar(DOM.userBlockchain.value, DOM.userAddress.value);
            DOM.notificationsAvatar.src = avatarUrl || defaultAvatar;
        }
        async function dismissNotification(id: string) {
            let response = await HttpPostJson(`/notifications/dismiss/${id}`, {}, DOM.csrfToken);
            if (response[0] !== 200) {
                LogError("Could not dismiss notification: " + id);
            }
            checkEmpty();
        }
        async function loadNotifications() {
            if (loading || !hasMore) return;
            loading = true;
            let response = await HttpGetJson(`/notifications/data?limit=${PAGE_SIZE}&offset=${offset}`);
            if (response[0] !== 200) {
                LogError("Could not fetch notifications");
                loading = false;
                return;
            }
            let notifications: UserNotification[] = response[1].notifications || [];
            if (notifications.length < PAGE_SIZE) {
                hasMore = false;
            }
            for (const notif of notifications) {
                let card = CreateNotificationCard(notif, dismissNotification);
                DOM.notificationsDiv.appendChild(card);
            }
            offset += notifications.length;
            loading = false;
            checkEmpty();
        }
        function checkEmpty() {
            let cards = DOM.notificationsDiv.querySelectorAll(".notificationCard");
            DOM.notificationsEmpty.style.display = cards.length === 0 ? "block" : "none";
        }
        async function markSeen() {
            await HttpPostJson("/notifications/seen", {}, DOM.csrfToken);
        }

        DOM.clearAllBtn.addEventListener("click", async () => {
            let response = await HttpPostJson("/notifications/clear", {}, DOM.csrfToken);
            if (response[0] !== 200) {
                LogError("Could not clear notifications");
                return;
            }
            DOM.notificationsDiv.innerHTML = "";
            hasMore = false;
            checkEmpty();
        });

        let scrollObserver = new IntersectionObserver((entries) => {
            if (entries[0].isIntersecting) {
                loadNotifications().then();
            }
        }, {rootMargin: "200px"});
        let sentinel = document.createElement("div");
        sentinel.id = "notificationsSentinel";
        DOM.notificationsDiv.after(sentinel);
        scrollObserver.observe(sentinel);

        renderNotificationsAvatar().then();
        loadNotifications().then();
        if (DOM.isCookieAuthenticated.value === "true") {
            markSeen().then();
        }
        ShowNotifications().then();
    }
})();
