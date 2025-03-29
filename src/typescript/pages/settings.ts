
window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/global.scss";
import "../../scss/pages/settings.scss";
import "../components/menu";
import DOMPurify from "dompurify";
import {HttpGetJson, HttpPostJson} from "../util/network";
import {LogError, LogInfo} from "../util/log";
import {createPopper, type Instance} from "@popperjs/core";
import {ShowDialogModal, ShowDialogModalHTML,} from "../components/modalDialog";
import {ShowModalYesNo, ShowModalYesNoHTML} from "../components/modalYesNo";
import {AIIsEnabled, AIIsModelEnabled} from "../services/ai";
import {ShowSavedToast} from "../components/toast";
import {ExpandAccordionByHash, InitTooltips} from "../util/bootstrap";


(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        let DOM = {
            baseDataDirectory: document.getElementById("baseDataDirectory")! as HTMLInputElement,
            baseDefaultDataDirectoryBtn: document.getElementById("baseDefaultDataDirectoryBtn")! as HTMLButtonElement,
            baseFullNodeDataDirectoryDiv: document.getElementById("baseFullNodeDataDirectoryDiv")! as HTMLDivElement,
            baseFullNodeCheckbox: document.getElementById("baseFullNodeCheckbox")! as HTMLInputElement,
            baseSaveDataDirectoryBtn: document.getElementById("baseSaveDataDirectoryBtn")! as HTMLButtonElement,
            baseIndexerProgressUncachedTail: document.getElementById("baseIndexerProgressUncachedTail")! as HTMLDivElement,
            baseIndexerCachedPercent: document.getElementById("baseIndexerCachedPercent")! as HTMLSpanElement,
            baseIndexerProgressCached: document.getElementById("baseIndexerProgressCached")! as HTMLDivElement,
            baseIndexerProgressUncachedHead: document.getElementById("baseIndexerProgressUncachedHead")! as HTMLDivElement,
            baseThrottle: document.getElementById("baseThrottle")! as HTMLInputElement,
            baseThrottleTooltip: document.getElementById("baseThrottleTooltip")! as HTMLElement,
            baseThrottleNumber: document.getElementById("baseThrottleNumber")! as HTMLDivElement,
            baseURL: document.getElementById("baseURL")! as HTMLInputElement,
            baseIndexerResetBtn: document.getElementById("baseIndexerResetBtn")! as HTMLButtonElement,
            csrfToken: document.getElementById("csrfToken")! as HTMLInputElement,
            defaultBaseURLBtn: document.getElementById("defaultBaseURLBtn")! as HTMLButtonElement,
            defaultUploadDirectoryBtn: document.getElementById("defaultUploadDirectoryBtn")! as HTMLButtonElement,
            indexerServer: document.getElementById("indexerServer")! as HTMLInputElement,
            indexerToken: document.getElementById("indexerToken")! as HTMLInputElement,
            indexerOnBatteryCheckbox: document.getElementById("indexerOnBatteryCheckbox")! as HTMLInputElement,
            ollamaTrafficLight: document.getElementById("ollamaTrafficLight")! as HTMLDivElement,
            ollamaModelTrafficLight: document.getElementById("ollamaModelTrafficLight")! as HTMLDivElement,
            postHistoryDays: document.getElementById("postHistoryDays")! as HTMLInputElement,
            saveBaseURLBtn: document.getElementById("saveBaseURLBtn")! as HTMLButtonElement,
            savePostHistoryDaysBtn: document.getElementById("savePostHistoryDaysBtn")! as HTMLButtonElement,
            saveUploadDirectoryBtn: document.getElementById("saveUploadDirectoryBtn")! as HTMLButtonElement,
            spiceometerCheck: document.getElementById("spiceometerCheck")! as HTMLInputElement,
            uploadDirectory: document.getElementById("uploadDirectory")! as HTMLInputElement,
            yourplaceTrafficLight: document.getElementById("yourplaceTrafficLight")! as HTMLDivElement,
            ipfsTrafficLight: document.getElementById("ipfsTrafficLight")! as HTMLDivElement,
            retestPortsBtn: document.getElementById("retestPortsBtn")! as HTMLButtonElement,
            ipfsPinningURL: document.getElementById("ipfsPinningURL")! as HTMLInputElement,
            ipfsPinningKey: document.getElementById("ipfsPinningKey")! as HTMLInputElement,
            pinataLI: document.getElementById("pinataLI")! as HTMLLIElement,
            web3LI: document.getElementById("web3LI")! as HTMLLIElement,
            //eternumLI: document.getElementById("eternumLI")! as HTMLLIElement,
            filebaseLi: document.getElementById("filebaseLI")! as HTMLLIElement,
            saveIpfsPinningBtn: document.getElementById("saveIpfsPinningBtn")! as HTMLButtonElement,
        }
        let popperInstance: Instance | null = null;

        async function init() {
            InitTooltips();

            try {
                await Promise.all([
                    getUploadDirectory(),
                    getBaseURL(),
                    getPostHistoryDays(),
                    getBaseIndexerProgress(),
                    getBaseThrottle(),
                    getBaseFullNode(),
                    getBaseDataDirectory(),
                    getSpiceometer(),
                    getOllamaEnabled(),
                    getOllamaModelEnabled(),
                    getIndexerOnBattery(),
                    getNetworkPorts(),
                    getIpfsPinning(),
                ]);
            } catch (error) {
                LogError("Error initializing settings page: " + error);
            }

            ExpandAccordionByHash();

            /* Cron Jobs */
            setInterval(getBaseIndexerProgress, 120000);
        }

        /* Getting Current Settings Values */
        async function getBaseDataDirectory() {
            let response = await HttpGetJson("/settings/base/dataDirectory");
            if (response[0] === 200) {
                DOM.baseDataDirectory.value = DOMPurify.sanitize(response[1].dataDirectory);
            }
        }
        async function getBaseFullNode() {
            let response = await HttpGetJson("/settings/base/fullNode");
            if (response[0] === 200) {
                DOM.baseFullNodeCheckbox.checked = response[1].baseFullNode;
                DOM.baseFullNodeDataDirectoryDiv.style.display = response[1].baseFullNode ? "block" : "none";
            }
        }
        async function getBaseURL() {
            let response = await HttpGetJson("/settings/base/url");
            if (response[0] === 200) {
                DOM.baseURL.value = DOMPurify.sanitize(response[1].baseURL);
            }
        }
        async function getBaseIndexerProgress() {
            let response = await HttpGetJson("/settings/base/indexerProgress");
            if (response[0] === 200) {
                let earliestBlock = response[1].earliestBlock;
                let tailBlock = response[1].tailBlock;
                let headBlock = response[1].headBlock;
                let latestBlock = response[1].latestBlock;
                LogInfo("Base Indexer Progress: " + earliestBlock + " " + tailBlock + " " + headBlock + " " + latestBlock);
                // Calculate the ranges
                const totalRange = latestBlock - earliestBlock;
                const earliestToTailRange = tailBlock - earliestBlock;
                const tailToHeadRange = headBlock - tailBlock;
                const headToLatestRange = latestBlock - headBlock;
                // Calculate percentages and round to nearest integer
                const tailPercentage = Math.round((earliestToTailRange / totalRange) * 100);
                const latestPercentage = Math.round((headToLatestRange / totalRange) * 100);
                const cachedPercentage = 100 - (tailPercentage + latestPercentage);
                DOM.baseIndexerProgressUncachedTail.style.width = tailPercentage + "%";
                DOM.baseIndexerProgressUncachedTail.ariaValueNow = tailPercentage.toString();
                DOM.baseIndexerProgressCached.style.width = cachedPercentage + "%";
                DOM.baseIndexerProgressCached.ariaValueNow = cachedPercentage.toString();
                DOM.baseIndexerCachedPercent.textContent = cachedPercentage.toString() + "%";
                if (cachedPercentage === 100) {
                    DOM.baseIndexerCachedPercent.classList.remove("progress-bar-animated");
                    DOM.baseIndexerProgressCached.classList.remove("progress-bar-striped");
                    DOM.baseIndexerProgressCached.classList.add("bg-success");
                } else {
                    DOM.baseIndexerCachedPercent.classList.add("progress-bar-animated");
                    DOM.baseIndexerProgressCached.classList.add("progress-bar-striped");
                    DOM.baseIndexerProgressCached.classList.remove("bg-success");
                }
                DOM.baseIndexerProgressUncachedHead.style.width = latestPercentage + "%";
                DOM.baseIndexerProgressUncachedHead.ariaValueNow = latestPercentage.toString();
            }
        }
        async function getBaseThrottle() {
            let response = await HttpGetJson("/settings/base/throttle");
            if (response[0] === 200) {
                const cleanThrottle = DOMPurify.sanitize(response[1].throttle);
                DOM.baseThrottle.value = cleanThrottle;
                DOM.baseThrottleNumber.textContent = cleanThrottle;
            } else {
                DOM.baseThrottleNumber.textContent = "Error ⚠️";
            }
        }
        async function getUploadDirectory() {
            let response = await HttpGetJson("/settings/uploadDirectory");
            if (response[0] === 200) {
                DOM.uploadDirectory.value = DOMPurify.sanitize(response[1].uploadDirectory);
            }
        }
        async function getPostHistoryDays() {
            let response = await HttpGetJson("/settings/post/history");
            if (response[0] === 200) {
                DOM.postHistoryDays.value = DOMPurify.sanitize(response[1].days);
            } else {
                DOM.postHistoryDays.value = "Error";
            }
        }
        async function getSpiceometer() {
            let ollamaEnabled = await AIIsEnabled();
            DOM.spiceometerCheck.checked = ollamaEnabled;
        }
        async function getOllamaEnabled() {
            let ollamaEnabled = await AIIsEnabled();
            if (ollamaEnabled) {
                DOM.ollamaTrafficLight.classList.remove("redLight");
                DOM.ollamaTrafficLight.classList.add("greenLight");
            } else {
                DOM.ollamaTrafficLight.classList.remove("greenLight");
                DOM.ollamaTrafficLight.classList.add("redLight");
            }
        }
        async function getOllamaModelEnabled() {
            let ollamaModelEnabled = await AIIsModelEnabled();
            if (ollamaModelEnabled) {
                DOM.ollamaModelTrafficLight.classList.remove("redLight");
                DOM.ollamaModelTrafficLight.classList.add("greenLight");
            } else {
                DOM.ollamaModelTrafficLight.classList.remove("greenLight");
                DOM.ollamaModelTrafficLight.classList.add("redLight");
            }
        }
        async function getIndexerOnBattery() {
            let response = await HttpGetJson("/settings/indexer/onBattery");
            if (response[0] === 200) {
                DOM.indexerOnBatteryCheckbox.checked = response[1].indexerOnBattery;
            } else {
                DOM.indexerOnBatteryCheckbox.checked = false;
            }
        }
        async function getNetworkPorts() {
            let response = await HttpGetJson("https://yourplace.network/ports");
            if (response[0] === 200) {
                if (response[1].port_4001) {
                    DOM.ipfsTrafficLight.classList.remove("redLight");
                    DOM.ipfsTrafficLight.classList.add("greenLight");
                } else {
                    DOM.ipfsTrafficLight.classList.remove("greenLight");
                    DOM.ipfsTrafficLight.classList.add("redLight");
                }

                if (response[1].port_42424) {
                    DOM.yourplaceTrafficLight.classList.remove("redLight");
                    DOM.yourplaceTrafficLight.classList.add("greenLight");
                } else {
                    DOM.yourplaceTrafficLight.classList.remove("greenLight");
                    DOM.yourplaceTrafficLight.classList.add("redLight");
                }
            } else {
                DOM.yourplaceTrafficLight.classList.remove("greenLight");
                DOM.yourplaceTrafficLight.classList.add("redLight");
                DOM.ipfsTrafficLight.classList.remove("greenLight");
                DOM.ipfsTrafficLight.classList.add("redLight");
                LogError("Failed to check network ports");
            }
        }
        async function getIpfsPinning() {
            let response = await HttpGetJson("/settings/content/ipfsPinning");
            if (response[0] === 200) {
                DOM.ipfsPinningURL.value = response[1].pinningURL;
                DOM.ipfsPinningKey.value = response[1].pinningKey;
            } else {
                ShowDialogModal("Failed to get IPFS Pinning settings");
            }
        }

        /* State Changing Settings Functions */
        async function setBaseDataDirectory() {
            const data = {
                dataDirectory: DOM.baseDataDirectory.value,
            }
            let response = await HttpPostJson("/settings/base/dataDirectory", data, DOM.csrfToken.value);
            if (response[0] === 200) {
                LogInfo("Base Data Directory Saved");
                ShowSavedToast();
            } else {
                DOM.baseDataDirectory.value = DOMPurify.sanitize(response[1].status);
            }
        }
        async function setBaseFullNode() {
            let baseFullNodeChecked = DOM.baseFullNodeCheckbox.checked;
            if (baseFullNodeChecked) {
                DOM.baseFullNodeDataDirectoryDiv.style.display = "block";
            } else {
                DOM.baseFullNodeDataDirectoryDiv.style.display = "none";
            }
            const data = {
                baseFullNode: baseFullNodeChecked
            }
            let response = await HttpPostJson("/settings/base/fullNode", data, DOM.csrfToken.value);
            if (response[0] === 200) {
                DOM.baseFullNodeCheckbox.checked = response[1].baseFullNode;
                ShowSavedToast();
            }
        }
        async function setBaseURL() {
            const data = {
                baseURL: DOM.baseURL.value,
            }
            let response = await HttpPostJson("/settings/base/url", data, DOM.csrfToken.value);
            if (response[0] === 200) {
                LogInfo("Base URL Saved");
                ShowSavedToast();
            } else {
                DOM.baseURL.value = DOMPurify.sanitize(response[1].status);
            }
        }
        async function setBaseThrottle() {
            const data = {
                throttle: DOM.baseThrottle.valueAsNumber,
            }
            let response = await HttpPostJson("/settings/base/throttle", data, DOM.csrfToken.value);
            if (response[0] === 200) {
                LogInfo("Base Throttle Saved");
                DOM.baseThrottleNumber.textContent = DOM.baseThrottle.value;
                ShowSavedToast();
            }
        }
        async function setUploadDirectory() {
            const data = {
                uploadDirectory: DOM.uploadDirectory.value,
            }
            let response = await HttpPostJson("/settings/uploadDirectory", data, DOM.csrfToken.value);
            if (response[0] === 200) {
                LogInfo("Upload Directory Saved");
                ShowSavedToast();
            } else {
                DOM.uploadDirectory.value = response[1].status;
            }
        }
        async function setDefaultBaseURL() {
            const data = {
                baseURL: "default",
            }
            let response = await HttpPostJson("/settings/base/url", data, DOM.csrfToken.value);
            if (response[0] === 200) {
                DOM.baseURL.value = response[1].defaultBaseURL;
                ShowSavedToast();
            }
            // Set the throttle back to 25 RPS
            DOM.baseThrottle.value = "25";
            await setBaseThrottle();
        }
        async function setDefaultBaseDataDirectory() {
            const data = {
                dataDirectory: "default",
            }
            let response = await HttpPostJson("/settings/base/dataDirectory", data, DOM.csrfToken.value);
            if (response[0] === 200) {
                DOM.baseDataDirectory.value = response[1].baseDataDirectory;
                ShowSavedToast();
            }
        }
        async function setDefaultUploadDirectory() {
            const data = {
                uploadDirectory: "default",
            }
            let response = await HttpPostJson("/settings/uploadDirectory", data, DOM.csrfToken.value);
            if (response[0] === 200) {
                DOM.uploadDirectory.value = response[1].defaultUploadDirectory;
                ShowSavedToast();
            }
        }
        async function setPostHistoryDays() {
            const data = {
                days: DOM.postHistoryDays.valueAsNumber,
            }
            let response = await HttpPostJson("/settings/post/history", data, DOM.csrfToken.value);
            if (response[0] === 200) {
                LogInfo("Base Post Cache Days Saved");
                ShowSavedToast();
            } else {
                DOM.postHistoryDays.value = "Error";
            }
        }
        async function setBaseIndexerReset() {
            const confirmed = await ShowModalYesNoHTML("⚠️ Are you sure you want to reset the indexer? ⚠️<br><br>This will delete cached YourPlace data and re-index everything<br><br>It will take a long time and download a lot of data<br><br>Your personal data, posts, and profile <u>will not</u> be deleted");
            if (confirmed) {
                let response = await HttpPostJson("/settings/base/indexerReset",
                    {indexerReset: true},
                    DOM.csrfToken.value);
                if (response[0] === 200) {
                    LogInfo("Base Indexer Reset");
                    ShowSavedToast();
                } else {
                    LogInfo("Base Indexer Reset Error");
                }
            }
        }
        async function setSpiceometer() {
            if (DOM.spiceometerCheck.checked) {
                let response = await HttpPostJson("/settings/ai/spiceometer", {enable: 1}, DOM.csrfToken.value);
                if (response[0] === 200) {
                    DOM.spiceometerCheck.checked = true;
                    ShowSavedToast();
                } else {
                    DOM.spiceometerCheck.checked = false;
                    ShowDialogModal("Could not enable Spice-o-Meter");
                }
            } else {
                let response = await HttpPostJson("/settings/ai/spiceometer", {enable: 0}, DOM.csrfToken.value);
                if (response[0] === 200) {
                    DOM.spiceometerCheck.checked = false;
                    ShowSavedToast();
                } else {
                    DOM.spiceometerCheck.checked = true;
                    ShowDialogModal("Could not disable Spice-o-Meter");
                }
            }
        }
        async function setIndexerOnBattery() {
            let response = await HttpPostJson("/settings/indexer/onBattery",
                {indexerOnBattery: DOM.indexerOnBatteryCheckbox.checked}, DOM.csrfToken.value);
            if (response[0] === 200) {
                LogInfo("Indexer On Battery Saved");
                ShowSavedToast();
            } else {
                LogError("Indexer On Battery Error");
            }
        }
        async function setIPFSPinning() {
            const data = {
                pinningURL: DOM.ipfsPinningURL.value,
                pinningKey: DOM.ipfsPinningKey.value,
            }
            let response = await HttpPostJson("/settings/content/ipfsPinning", data, DOM.csrfToken.value);
            if (response[0] === 200) {
                ShowSavedToast();
            } else {
                ShowDialogModal(response[1].status);
            }
        }

        /* Throttle Slider */
        const getThumbElement = (): HTMLElement => {
            // Create a virtual reference for the thumb position
            const thumbPosition = (Number(DOM.baseThrottle.value) - Number(DOM.baseThrottle.min)) /
                (Number(DOM.baseThrottle.max) - Number(DOM.baseThrottle.min));

            const inputRect = DOM.baseThrottle.getBoundingClientRect();
            const thumbWidth = 16; // Default Bootstrap thumb width

            return {
                getBoundingClientRect: () => ({
                    width: thumbWidth,
                    height: thumbWidth,
                    top: inputRect.top,
                    bottom: inputRect.bottom,
                    left: inputRect.left + (thumbPosition * (inputRect.width - thumbWidth)),
                    right: inputRect.left + (thumbPosition * (inputRect.width - thumbWidth)) + thumbWidth,
                    x: inputRect.left + (thumbPosition * (inputRect.width - thumbWidth)),
                    y: inputRect.top,
                    toJSON: () => {
                    }
                }),
                clientWidth: thumbWidth,
                clientHeight: thumbWidth
            } as unknown as HTMLElement;
        }
        const showTooltip = () => {
            DOM.baseThrottleTooltip.style.display = 'block';

            if (!popperInstance) {
                popperInstance = createPopper(getThumbElement(), DOM.baseThrottleTooltip, {
                    placement: 'top',
                    modifiers: [
                        {
                            name: 'offset',
                            options: {
                                offset: [0, 8],
                            },
                        },
                    ],
                });
            }
        }
        const hideTooltip = () => {
            DOM.baseThrottleTooltip.style.display = 'none';
            if (popperInstance) {
                popperInstance.destroy();
                popperInstance = null;
            }
        }
        const updateTooltip = () => {
            document.querySelector('.throttle-tooltip-inner')!.textContent = DOM.baseThrottle.value;
            if (popperInstance) {
                popperInstance.state.elements.reference = getThumbElement();
                popperInstance.update();
            }
        }
        DOM.baseThrottle.addEventListener("input", () => {
            showTooltip();
            updateTooltip();
        });
        DOM.baseThrottle.addEventListener("mousedown", showTooltip);
        DOM.baseThrottle.addEventListener("touchstart", showTooltip);
        DOM.baseThrottle.addEventListener("mouseup", hideTooltip);
        DOM.baseThrottle.addEventListener("mouseleave", hideTooltip);
        DOM.baseThrottle.addEventListener("touchend", hideTooltip);

        /* Event Listeners */
        DOM.baseDataDirectory!.addEventListener("change", setBaseDataDirectory);
        DOM.baseDefaultDataDirectoryBtn!.addEventListener("click", setDefaultBaseDataDirectory);
        DOM.baseFullNodeCheckbox!.addEventListener("change", setBaseFullNode);
        DOM.baseThrottle!.addEventListener("change", setBaseThrottle);
        DOM.baseSaveDataDirectoryBtn!.addEventListener("click", setBaseDataDirectory);
        DOM.baseIndexerResetBtn!.addEventListener("click", setBaseIndexerReset);
        DOM.defaultBaseURLBtn!.addEventListener("click", setDefaultBaseURL);
        DOM.defaultUploadDirectoryBtn!.addEventListener("click", setDefaultUploadDirectory);
        DOM.saveBaseURLBtn!.addEventListener("click", setBaseURL);
        DOM.saveUploadDirectoryBtn!.addEventListener("click", setUploadDirectory);
        DOM.savePostHistoryDaysBtn!.addEventListener("click", setPostHistoryDays);
        DOM.spiceometerCheck!.addEventListener("change", setSpiceometer);
        DOM.indexerOnBatteryCheckbox!.addEventListener("change", setIndexerOnBattery);
        DOM.retestPortsBtn!.addEventListener("click", getNetworkPorts);
        DOM.pinataLI!.addEventListener("click", function(e) {
            ShowDialogModalHTML("Please create an account and secret from <a href='https://app.pinata.cloud/' target='_blank'>Pinata here</a><br><br>Then add your \"<b>Gateway URL</b>\" and \"<b>JWT (secret access token)</b>\" to the IPFS Pinning settings page");
        });
        DOM.web3LI!.addEventListener("click", function(e) {
            DOM.ipfsPinningURL.value = "https://api.web3.storage";
            ShowDialogModalHTML("Please create an account and secret from <a href='https://web3.storage/' target='_blank'>Web3.Storage here</a><br><br>Then add your \"<b>Secret Access Token</b>\" to the IPFS Pinning settings page");
        });
        DOM.filebaseLi!.addEventListener("click", function(e) {
            DOM.ipfsPinningURL.value = "https://api.filebase.io/v1/ipfs";
            ShowDialogModalHTML("Please create an account at <a href='https://console.filebase.com/' target='_blank'>Filebase here</a><br><br>Then create a Bucket and generate a \"<b>Token</b>\" for that bucket, and add it to the IPFS Pinning settings page");
        });
        //DOM.eternumLI!.addEventListener("click", function(e) {});
        DOM.saveIpfsPinningBtn.addEventListener("click", setIPFSPinning);

        init().then();
    }
})();