window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/pages/mentalHealth.scss";
import "../components/menu";
import {ShowDialogModalHTML} from "../components/modalDialog";
import {LogInfo} from "../util/log";
import {XSSSanitizeTextUrl} from "../util/security";

interface TipData {
    hash: string;
    text: string;
}

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        let DOM = {
            emergencyBtn: document.getElementById("emergencyBtn")! as HTMLButtonElement,
            pause: document.getElementById("pause")! as HTMLButtonElement,
            play: document.getElementById("play")! as HTMLButtonElement,
            refreshBtn: document.getElementById("refreshBtn")! as HTMLButtonElement,
            shareBtn: document.getElementById("shareBtn")! as HTMLButtonElement,
            tipDiv: document.getElementById("tip")! as HTMLDivElement,
            tipHashInput: document.getElementById("tipHash") as HTMLInputElement,
        }
        let currentTip: TipData | null = null;
        let intervalId: NodeJS.Timeout;
        let intervalTimeout = 30000;
        let paused = false;
        let recentTipIndices: number[] = [];
        let tips: TipData[] = [];

        function displayTip(tip: TipData) {
            currentTip = tip;
            DOM.tipDiv.innerHTML = XSSSanitizeTextUrl(tip.text);
            const url = new URL(window.location.href);
            url.searchParams.set("tip", tip.hash);
            history.replaceState(null, "", url.toString());
        }
        function emergency() {
            ShowDialogModalHTML(
                "<div class='emergencyModal'>" +
                "If you are having an Emergency call:" +
                "<br>&emsp;<a href='tel:911' class='obviousLink'>911 <i class='bi bi-telephone-outbound-fill'></i></a> (Americas)" +
                "<br>&emsp;<a href='tel:112' class='obviousLink'>112 <i class='bi bi-telephone-outbound-fill'></i></a> (Europe, Asia, Africa)" +
                "<br>&emsp;<a href='tel:000' class='obviousLink'>000 <i class='bi bi-telephone-outbound-fill'></i></a> (Australia)" +
                "<br>&emsp;<a href='tel:999' class='obviousLink'>999 <i class='bi bi-telephone-outbound-fill'></i></a> (Asia, Africa)" +
                "<br>&emsp;<a href='https://en.wikipedia.org/wiki/List_of_emergency_telephone_numbers' class='obviousLink' target='_blank'>Your country</a> emergency number may be different" +
                "<hr />" +
                "If you're feeling suicidal or are in emotional distress, please seek help:" +
                "<br>&emsp;<b>Suicide & Crisis Lifeline:</b> <a href='tel:988' class='obviousLink'>988 <i class='bi bi-telephone-outbound-fill'></i></a>" +
                "<br>&emsp;<b>Crisis Text Line:</b> Text HOME to <a href='sms:741741?body=HOME' class='obviousLink'>741741 <i class='bi bi-telephone-outbound-fill'></i></a>" +
                "<br>&emsp;<b>Crisis Online Chat:</b> <a href='https://chat.988lifeline.org/' class='obviousLink' target='_blank'>start a chat <i class='bi bi-chat-dots-fill'></i></a>" +
                "<br>&emsp;<b>Veterans Crisis Line:</b> <a href='tel:18002738255' class='obviousLink'>1-800-273-8255 <i class='bi bi-telephone-outbound-fill'></i></a> Press 1" +
                "<hr />" +
                "If you need general mental help, you can find more at the NIMH:" +
                "<br>&emsp;<a href='https://www.nimh.nih.gov/health/find-help' class='obviousLink' target='_blank'>NIMH Find Help <i class='bi bi-box-arrow-up-right'></i></a>" +
                "</div>"
            );
        }
        function findTipByHash(hash: string): TipData | null {
            for (const tip of tips) {
                if (tip.hash === hash) {
                    return tip;
                }
            }
            return null;
        }
        function pause() {
            LogInfo("Pause");
            if (intervalId) {
                clearInterval(intervalId);
            }
            paused = true;
            DOM.pause.classList.add("pressed");
            DOM.play.classList.remove("pressed");
        }
        function play() {
            LogInfo("Play");
            if (paused) {
                if (intervalId) {
                    clearInterval(intervalId);
                }
                intervalId = setInterval(reload, intervalTimeout);
                paused = false;
                DOM.play.classList.add("pressed");
                DOM.pause.classList.remove("pressed");
            }
        }
        function randomInt(limit: number): number {
            return Math.floor(Math.random() * limit);
        }
        function reload() {
            let newIndex = randomInt(tips.length);
            if (recentTipIndices.length >= 20) {
                recentTipIndices.pop();
            }
            if (recentTipIndices.includes(newIndex)) {
                reload();
                return;
            } else {
                recentTipIndices.push(newIndex);
                displayTip(tips[newIndex]);
            }
        }
        function shareTip() {
            if (currentTip) {
                const url = new URL(window.location.href);
                url.searchParams.set("tip", currentTip.hash);
                const shareUrl = url.toString();
                navigator.clipboard.writeText(shareUrl).then(() => {
                    ShowDialogModalHTML("Link copied to clipboard!<br><br><small>" + shareUrl + "</small>");
                }).catch(() => {
                    ShowDialogModalHTML("Copy this link:<br><br><input type='text' value='" + shareUrl + "' style='width:100%' readonly onclick='this.select()'>");
                });
            }
        }
        function startWithTips() {
            const serverTipHash = DOM.tipHashInput?.value || "";
            if (serverTipHash) {
                const linkedTip = findTipByHash(serverTipHash);
                if (linkedTip) {
                    displayTip(linkedTip);
                    const tipIndex = tips.indexOf(linkedTip);
                    if (tipIndex >= 0) {
                        recentTipIndices.push(tipIndex);
                    }
                    pause();
                    return;
                }
            }
            displayTip(tips[randomInt(tips.length)]);
            intervalId = setInterval(reload, intervalTimeout);
            DOM.play.classList.add("pressed");
        }

        fetch("/mentalHealth/tips")
            .then(response => response.json())
            .then((data: TipData[]) => {
                tips = data;
                startWithTips();
            })
            .catch(error => {
                LogInfo("Failed to load tips: " + error);
                DOM.tipDiv.innerHTML = "Unable to load tips. Please refresh the page.";
            });
        DOM.emergencyBtn.addEventListener("click", emergency);
        DOM.pause.addEventListener("click", pause);
        DOM.play.addEventListener("click", play);
        DOM.refreshBtn.addEventListener("click", () => {
            if (intervalId) {
                clearInterval(intervalId);
            }
            reload();
            if (!paused) {
                intervalId = setInterval(reload, intervalTimeout);
            }
        });
        DOM.shareBtn.addEventListener("click", shareTip);
    }
})();
