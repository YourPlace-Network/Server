window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/modalProfileDetails.scss";
import {HttpGetJson} from "../util/network";
import {getBlockchainIconPath} from "../util/domFactory";
import {EthosGetScore} from "../services/ethos";
import {IsValidBaseAddress} from "../util/security";

export function ShowModalProfileDetails(blockchain: string, address: string) {
    const modal = document.getElementById("modalProfileDetails") as HTMLDivElement;
    const blockchainIcon = document.getElementById("profileDetailBlockchainIcon") as HTMLImageElement;
    const blockchainName = document.getElementById("profileDetailBlockchainName") as HTMLSpanElement;
    const balanceEl = document.getElementById("profileDetailBalance") as HTMLSpanElement;
    const balanceUSDEl = document.getElementById("profileDetailBalanceUSD") as HTMLSpanElement;
    const ethosRow = document.getElementById("profileDetailEthosRow") as HTMLDivElement;
    const ethosEl = document.getElementById("profileDetailEthos") as HTMLSpanElement;
    const iconPath = getBlockchainIconPath(blockchain);
    if (iconPath) {
        blockchainIcon.src = iconPath;
        blockchainIcon.alt = blockchain;
    }
    blockchainName.textContent = blockchain.charAt(0).toUpperCase() + blockchain.slice(1);
    const spinner = '<span class="spinner-border spinner-border-sm"></span>';
    balanceEl.innerHTML = spinner;
    balanceUSDEl.innerHTML = spinner;
    ethosRow.style.display = "none";
    ethosEl.textContent = "--";
    const bsModal = new window.bootstrap.Modal(modal, {});
    bsModal.show();
    HttpGetJson(`/profile/details/${blockchain}/${address}`).then((response) => {
        if (response[0] === 200 && response[1]) {
            const data = response[1];
            const balance = parseFloat(data.balance);
            const balanceUSD = parseFloat(data.balanceUSD);
            const symbol = data.symbol || "";
            balanceEl.textContent = (isNaN(balance) ? "0" : balance.toFixed(4)) + (symbol ? " " + symbol : "");
            balanceUSDEl.textContent = isNaN(balanceUSD) ? "$0.00" : "$" + balanceUSD.toFixed(2);
        } else {
            balanceEl.textContent = "--";
            balanceUSDEl.textContent = "--";
        }
    });
    if (IsValidBaseAddress(address)) {
        EthosGetScore(address).then((result) => {
            if (result !== null) {
                ethosRow.style.display = "flex";
                const link = document.createElement("a");
                link.href = "https://app.ethos.network/profile/" + encodeURIComponent(address);
                link.target = "_blank";
                link.rel = "noopener noreferrer";
                link.textContent = result.score + " (" + result.label + ")";
                ethosEl.textContent = "";
                ethosEl.appendChild(link);
            }
        });
    }
}
