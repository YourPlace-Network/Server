import {HttpGetJson} from "../util/network";
import {WalletAcceptWelcomeNFT} from "../util/blockchain/wallet";

export function InitWelcomeNFT() {
    const DOM = {
        root: document.getElementById("welcomeNFT"),
        status: document.getElementById("welcomeNFTStatus")!,
        accept: document.getElementById("welcomeNFTAccept") as HTMLButtonElement,
        guest: document.getElementById("isGuest") as HTMLInputElement,
        csrf: document.getElementById("csrfToken") as HTMLInputElement,
    };
    if (!DOM.root || DOM.guest?.value !== "false") return;
    let approving = false;
    let timer: ReturnType<typeof setTimeout>;
    async function refresh() {
        if (approving) return;
        const [status, response] = await HttpGetJson("/nft/welcome");
        const grant = response?.grants?.[0];
        if (status !== 200 || !grant) return;
        DOM.root!.classList.remove("hidden");
        DOM.accept.classList.toggle("hidden", grant.stage !== "awaiting_recipient" || grant.approvalReceived);
        DOM.status.textContent = grant.stage === "delivered" ? "Welcome NFT delivered" : (grant.approvalReceived ? "Welcome NFT: delivery queued" : "Welcome NFT: " + grant.stage.replaceAll("_", " "));
        if (grant.stage !== "delivered") timer = setTimeout(refresh, 30000);
    }
    DOM.accept.addEventListener("click", async () => {
        approving = true;
        clearTimeout(timer);
        DOM.accept.disabled = true;
        DOM.status.textContent = "Awaiting wallet approval";
        const accepted = await WalletAcceptWelcomeNFT(DOM.csrf.value);
        approving = false;
        DOM.accept.disabled = false;
        DOM.status.textContent = accepted ? "Delivery queued" : "Approval not completed";
        if (accepted) DOM.accept.classList.add("hidden");
        timer = setTimeout(refresh, 30000);
    });
    window.addEventListener("pagehide", () => clearTimeout(timer), {once: true});
    void refresh();
}
