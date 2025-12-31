window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/global.scss";
import "../../scss/pages/settings.scss";
import "../components/menu";
import DOMPurify from "dompurify";
import {HttpGetJson, HttpPostJson} from "../util/network";
import {LogError, LogInfo} from "../util/log";
import {createPopper, type Instance} from "@popperjs/core";
import {DisableDialogModalOkBtn, EnableDialogModalOkBtn, ShowDialogModal, ShowDialogModalHTML} from "../components/modalDialog";
import {ShowModalYesNoHTML} from "../components/modalYesNo";
import {AIIsEnabled, AIIsModelEnabled} from "../services/ai";
import {ShowSavedToast, ShowToast} from "../components/toast";
import {ExpandAccordionByHash, InitTooltips} from "../util/bootstrap";
import {Sleep} from "../util/time";

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        let DOM = {
            baseDataDirectory: document.getElementById("baseDataDirectory")! as HTMLInputElement,
            baseDefaultDataDirectoryBtn: document.getElementById("baseDefaultDataDirectoryBtn")! as HTMLButtonElement,
            baseFullNodeDataDirectoryDiv: document.getElementById("baseFullNodeDataDirectoryDiv")! as HTMLDivElement,
            baseFullNodeCheckbox: document.getElementById("baseFullNodeCheckbox")! as HTMLInputElement,
            baseSaveDataDirectoryBtn: document.getElementById("baseSaveDataDirectoryBtn")! as HTMLButtonElement,
            baseIndexerBlocksIndexed: document.getElementById("baseIndexerBlocksIndexed")! as HTMLSpanElement,
            baseIndexerBlocksTotal: document.getElementById("baseIndexerBlocksTotal")! as HTMLSpanElement,
            baseIndexerCachedPercent: document.getElementById("baseIndexerCachedPercent")! as HTMLSpanElement,
            baseIndexerProgressCached: document.getElementById("baseIndexerProgressCached")! as HTMLDivElement,
            baseIndexerProgressUncachedHead: document.getElementById("baseIndexerProgressUncachedHead")! as HTMLDivElement,
            baseIndexerProgressUncachedTail: document.getElementById("baseIndexerProgressUncachedTail")! as HTMLDivElement,
            baseThrottle: document.getElementById("baseThrottle")! as HTMLInputElement,
            baseThrottleTooltip: document.getElementById("baseThrottleTooltip")! as HTMLElement,
            baseThrottleNumber: document.getElementById("baseThrottleNumber")! as HTMLDivElement,
            baseURL: document.getElementById("baseURL")! as HTMLInputElement,
            baseIndexerResetBtn: document.getElementById("baseIndexerResetBtn")! as HTMLButtonElement,
            baseIndexerCatchUpBtn: document.getElementById("baseIndexerCatchUpBtn")! as HTMLButtonElement,
            baseCatchUpFullBtn: document.getElementById("baseCatchUpFullBtn")! as HTMLButtonElement,
            baseCatchUpHelpBtn: document.getElementById("baseCatchUpHelpBtn")! as HTMLButtonElement,
            csrfToken: document.getElementById("csrfToken")! as HTMLInputElement,
            defaultBaseURLBtn: document.getElementById("defaultBaseURLBtn")! as HTMLButtonElement,
            defaultUploadDirectoryBtn: document.getElementById("defaultUploadDirectoryBtn")! as HTMLButtonElement,
            helperVersionText: document.getElementById("helperVersionText")! as HTMLSpanElement,
            indexerOnBatteryCheckbox: document.getElementById("indexerOnBatteryCheckbox")! as HTMLInputElement,
            indexerRunCheckbox: document.getElementById("indexerRunCheckbox")! as HTMLInputElement,
            indexerStatusText: document.getElementById("indexerStatusText")! as HTMLSpanElement,
            ollamaTrafficLight: document.getElementById("ollamaTrafficLight")! as HTMLDivElement,
            ollamaModelTrafficLight: document.getElementById("ollamaModelTrafficLight")! as HTMLDivElement,
            saveBaseURLBtn: document.getElementById("saveBaseURLBtn")! as HTMLButtonElement,
            saveUploadDirectoryBtn: document.getElementById("saveUploadDirectoryBtn")! as HTMLButtonElement,
            spiceometerCheck: document.getElementById("spiceometerCheck")! as HTMLInputElement,
            uploadDirectory: document.getElementById("uploadDirectory")! as HTMLInputElement,
            yourplaceTrafficLight: document.getElementById("yourplaceTrafficLight")! as HTMLDivElement,
            ipfsTrafficLight: document.getElementById("ipfsTrafficLight")! as HTMLDivElement,
            retestPortsBtn: document.getElementById("retestPortsBtn")! as HTMLButtonElement,
            ipfsPinningURL: document.getElementById("ipfsPinningURL")! as HTMLInputElement,
            ipfsPinningKey: document.getElementById("ipfsPinningKey")! as HTMLInputElement,
            pinataLI: document.getElementById("pinataLI")! as HTMLLIElement,
            saveIpfsPinningBtn: document.getElementById("saveIpfsPinningBtn")! as HTMLButtonElement,
            removeIpfsPinningBtn: document.getElementById("removeIpfsPinningBtn")! as HTMLButtonElement,
            ipfsGatewayURL: document.getElementById("ipfsGatewayURL")! as HTMLInputElement,
            defaultIpfsGatewayBtn: document.getElementById("defaultIpfsGatewayBtn")! as HTMLButtonElement,
            saveIpfsGatewayBtn: document.getElementById("saveIpfsGatewayBtn")! as HTMLButtonElement,
            contentAccordion: document.getElementById("contentAccordion")! as HTMLDivElement,
            privacyAccordion: document.getElementById("privacyAccordion")! as HTMLDivElement,
            networkingAccordion: document.getElementById("networkingAccordion")! as HTMLDivElement,
            blockchainAccordion: document.getElementById("blockchainAccordion")! as HTMLDivElement,
            serverDebugModeCheckbox: document.getElementById("serverDebugModeCheckbox")! as HTMLInputElement,
            serverUpdateBtn: document.getElementById("serverUpdateBtn")! as HTMLButtonElement,
            serverUninstallBtn: document.getElementById("serverUninstallBtn")! as HTMLButtonElement,
            serverVersionText: document.getElementById("serverVersionText")! as HTMLSpanElement,
            serverLogsViewBtn: document.getElementById("serverLogsViewBtn")! as HTMLButtonElement,
            helperLogsViewBtn: document.getElementById("helperLogsViewBtn")! as HTMLButtonElement,
            logsView: document.getElementById("logsView")! as HTMLDivElement,
            runtimeEnvVarsText: document.getElementById("runtimeEnvVarsText")! as HTMLDivElement,
            runtimeFlagsText: document.getElementById("runtimeFlagsText")! as HTMLSpanElement,
            torHiddenServiceCheck: document.getElementById("torHiddenServiceCheck")! as HTMLInputElement,
            collapseContent: document.getElementById("collapseContent")! as HTMLDivElement,
            collapseBlockchain: document.getElementById("collapseBlockchain")! as HTMLDivElement,
            collapseBase: document.getElementById("collapseBase")! as HTMLDivElement,
            collapseServerInfo: document.getElementById("collapseServerInfo")! as HTMLDivElement,
            collapseNetworking: document.getElementById("collapseNetworking")! as HTMLDivElement,
            // Algorand DOM elements
            algoURL: document.getElementById("algoURL")! as HTMLInputElement,
            algoThrottle: document.getElementById("algoThrottle")! as HTMLInputElement,
            algoThrottleTooltip: document.getElementById("algoThrottleTooltip")! as HTMLElement,
            algoThrottleNumber: document.getElementById("algoThrottleNumber")! as HTMLDivElement,
            algoIndexerBlocksIndexed: document.getElementById("algoIndexerBlocksIndexed")! as HTMLSpanElement,
            algoIndexerBlocksTotal: document.getElementById("algoIndexerBlocksTotal")! as HTMLSpanElement,
            algoIndexerCachedPercent: document.getElementById("algoIndexerCachedPercent")! as HTMLSpanElement,
            algoIndexerProgressCached: document.getElementById("algoIndexerProgressCached")! as HTMLDivElement,
            algoIndexerProgressUncachedHead: document.getElementById("algoIndexerProgressUncachedHead")! as HTMLDivElement,
            algoIndexerProgressUncachedTail: document.getElementById("algoIndexerProgressUncachedTail")! as HTMLDivElement,
            algoIndexerResetBtn: document.getElementById("algoIndexerResetBtn")! as HTMLButtonElement,
            algoIndexerCatchUpBtn: document.getElementById("algoIndexerCatchUpBtn")! as HTMLButtonElement,
            algoCatchUpFullBtn: document.getElementById("algoCatchUpFullBtn")! as HTMLButtonElement,
            algoCatchUpHelpBtn: document.getElementById("algoCatchUpHelpBtn")! as HTMLButtonElement,
            defaultAlgoURLBtn: document.getElementById("defaultAlgoURLBtn")! as HTMLButtonElement,
            saveAlgoURLBtn: document.getElementById("saveAlgoURLBtn")! as HTMLButtonElement,
            collapseAlgo: document.getElementById("collapseAlgo")! as HTMLDivElement,
            baseIndexerRunCheckbox: document.getElementById("baseIndexerRunCheckbox")! as HTMLInputElement,
            algoIndexerRunCheckbox: document.getElementById("algoIndexerRunCheckbox")! as HTMLInputElement,
            baseIndexerStatusLight: document.getElementById("baseIndexerStatusLight")! as HTMLDivElement,
            algoIndexerStatusLight: document.getElementById("algoIndexerStatusLight")! as HTMLDivElement,
            collapseServices: document.getElementById("collapseServices")! as HTMLDivElement,
            collapseXcom: document.getElementById("collapseXcom")! as HTMLDivElement,
            removeXcomCredentialsBtn: document.getElementById("removeXcomCredentialsBtn")! as HTMLButtonElement,
            saveXcomCredentialsBtn: document.getElementById("saveXcomCredentialsBtn")! as HTMLButtonElement,
            testXcomCredentialsBtn: document.getElementById("testXcomCredentialsBtn")! as HTMLButtonElement,
            xcomAccessToken: document.getElementById("xcomAccessToken")! as HTMLInputElement,
            xcomAccessTokenSecret: document.getElementById("xcomAccessTokenSecret")! as HTMLInputElement,
            xcomApiKey: document.getElementById("xcomApiKey")! as HTMLInputElement,
            xcomApiSecret: document.getElementById("xcomApiSecret")! as HTMLInputElement,
            xcomCrossPostCheckbox: document.getElementById("xcomCrossPostCheckbox")! as HTMLInputElement,
            xcomFeedAggregationCheckbox: document.getElementById("xcomFeedAggregationCheckbox")! as HTMLInputElement,
            xcomStatusLight: document.getElementById("xcomStatusLight")! as HTMLDivElement,
            xcomScrapeCredentialsDiv: document.getElementById("xcomScrapeCredentialsDiv")! as HTMLDivElement,
            xcomScrapeEmail: document.getElementById("xcomScrapeEmail")! as HTMLInputElement,
            xcomScrapeUsername: document.getElementById("xcomScrapeUsername")! as HTMLInputElement,
            xcomScrapePassword: document.getElementById("xcomScrapePassword")! as HTMLInputElement,
            saveXcomScrapeCredentialsBtn: document.getElementById("saveXcomScrapeCredentialsBtn")! as HTMLButtonElement,
            removeXcomScrapeCredentialsBtn: document.getElementById("removeXcomScrapeCredentialsBtn")! as HTMLButtonElement,
            testXcomScrapeCredentialsBtn: document.getElementById("testXcomScrapeCredentialsBtn")! as HTMLButtonElement,
            xcomScrapeStatusLight: document.getElementById("xcomScrapeStatusLight")! as HTMLDivElement,
        }
        const DEFAULT_ALGO_THROTTLE = "60";
        const DEFAULT_BASE_THROTTLE = "5";
        let algoPopperInstance: Instance | null = null;
        let popperInstance: Instance | null = null;

        async function init() {
            InitTooltips();
            ExpandAccordionByHash();

            setInterval(getBaseIndexerProgress, 300000);
            setInterval(getAlgoIndexerProgress, 300000);
            setInterval(getIndexerStatus, 6000);
            setInterval(getIndexerRunning, 6000);
            setInterval(getBaseIndexerStatus, 6000);
            setInterval(getAlgoIndexerStatus, 6000);
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
                // Handle invalid data
                if (isNaN(earliestBlock) || isNaN(tailBlock) || isNaN(headBlock) || isNaN(latestBlock)) {
                    console.error("Invalid Base indexer data received from server");
                    return;
                }
                // Calculate the ranges
                const totalRange = latestBlock - earliestBlock;
                // Prevent division by zero
                if (totalRange <= 0) {
                    console.error("Invalid Base indexer block range: ", {earliestBlock, latestBlock});
                    return;
                }
                const earliestToTailRange = tailBlock - earliestBlock;
                const tailToHeadRange = headBlock - tailBlock;
                const headToLatestRange = latestBlock - headBlock;
                // Calculate percentages and round to nearest integer
                const tailPercentage = Math.max(0, Math.min(100, Math.round((earliestToTailRange / totalRange) * 100)));
                const latestPercentage = Math.max(0, Math.min(100, Math.round((headToLatestRange / totalRange) * 100)));
                const cachedPercentage = Math.max(0, Math.min(100, 100 - (tailPercentage + latestPercentage)));
                // Update DOM elements with the calculated values
                DOM.baseIndexerProgressUncachedTail.style.width = tailPercentage + "%";
                DOM.baseIndexerProgressUncachedTail.setAttribute("aria-valuenow", tailPercentage.toString());
                DOM.baseIndexerProgressCached.style.width = cachedPercentage + "%";
                DOM.baseIndexerProgressCached.setAttribute("aria-valuenow", cachedPercentage.toString());
                DOM.baseIndexerProgressUncachedHead.style.width = latestPercentage + "%";
                DOM.baseIndexerProgressUncachedHead.setAttribute("aria-valuenow", latestPercentage.toString());
                DOM.baseIndexerCachedPercent.textContent = cachedPercentage.toString() + "%";
                DOM.baseIndexerBlocksIndexed.textContent = tailToHeadRange.toLocaleString();
                DOM.baseIndexerBlocksTotal.textContent = totalRange.toLocaleString();
                if (cachedPercentage === 100) {
                    DOM.baseIndexerProgressCached.classList.remove("progress-bar-striped", "progress-bar-animated");
                    DOM.baseIndexerProgressCached.classList.add("bg-success");
                    DOM.baseIndexerCachedPercent.classList.add("indexerComplete");
                } else {
                    DOM.baseIndexerProgressCached.classList.add("progress-bar-striped", "progress-bar-animated");
                    DOM.baseIndexerProgressCached.classList.remove("bg-success");
                    DOM.baseIndexerCachedPercent.classList.remove("indexerComplete");
                }
            } else {
                console.error("Failed to get Base indexer progress");
            }
        }
        async function getBaseThrottle() {
            let response = await HttpGetJson("/settings/base/throttle");
            if (response[0] === 200) {
                const cleanThrottle = DOMPurify.sanitize(response[1].throttle);
                DOM.baseThrottle.value = cleanThrottle;
                DOM.baseThrottleNumber.textContent = cleanThrottle;
                DOM.baseThrottle.disabled = response[1].isDefault;
            } else {
                DOM.baseThrottle.value = DEFAULT_BASE_THROTTLE;
                DOM.baseThrottleNumber.textContent = DEFAULT_BASE_THROTTLE;
                DOM.baseThrottle.disabled = true;
            }
        }
        // Algorand Getters
        async function getAlgoURL() {
            let response = await HttpGetJson("/settings/algorand/url");
            if (response[0] === 200) {
                DOM.algoURL.value = DOMPurify.sanitize(response[1].algoURL);
            }
        }
        async function getAlgoThrottle() {
            let response = await HttpGetJson("/settings/algorand/throttle");
            if (response[0] === 200) {
                const cleanThrottle = DOMPurify.sanitize(response[1].throttle);
                DOM.algoThrottle.value = cleanThrottle;
                DOM.algoThrottleNumber.textContent = cleanThrottle;
                DOM.algoThrottle.disabled = response[1].isDefault;
            } else {
                DOM.algoThrottle.value = DEFAULT_ALGO_THROTTLE;
                DOM.algoThrottleNumber.textContent = DEFAULT_ALGO_THROTTLE;
                DOM.algoThrottle.disabled = true;
            }
        }
        async function getAlgoIndexerProgress() {
            let response = await HttpGetJson("/settings/algorand/indexerProgress");
            if (response[0] === 200) {
                let earliestBlock = response[1].earliestBlock;
                let tailBlock = response[1].tailBlock;
                let headBlock = response[1].headBlock;
                let latestBlock = response[1].latestBlock;
                LogInfo("Algorand Indexer Progress: " + earliestBlock + " " + tailBlock + " " + headBlock + " " + latestBlock);
                if (isNaN(earliestBlock) || isNaN(tailBlock) || isNaN(headBlock) || isNaN(latestBlock)) {
                    console.error("Invalid Algorand indexer data received from server");
                    return;
                }
                const totalRange = latestBlock - earliestBlock;
                if (totalRange <= 0) {
                    console.error("Invalid Algorand indexer block range: ", {earliestBlock, latestBlock});
                    return;
                }
                const earliestToTailRange = tailBlock - earliestBlock;
                const tailToHeadRange = headBlock - tailBlock;
                const headToLatestRange = latestBlock - headBlock;
                const tailPercentage = Math.max(0, Math.min(100, Math.round((earliestToTailRange / totalRange) * 100)));
                const latestPercentage = Math.max(0, Math.min(100, Math.round((headToLatestRange / totalRange) * 100)));
                const cachedPercentage = Math.max(0, Math.min(100, 100 - (tailPercentage + latestPercentage)));
                DOM.algoIndexerProgressUncachedTail.style.width = tailPercentage + "%";
                DOM.algoIndexerProgressUncachedTail.setAttribute("aria-valuenow", tailPercentage.toString());
                DOM.algoIndexerProgressCached.style.width = cachedPercentage + "%";
                DOM.algoIndexerProgressCached.setAttribute("aria-valuenow", cachedPercentage.toString());
                DOM.algoIndexerProgressUncachedHead.style.width = latestPercentage + "%";
                DOM.algoIndexerProgressUncachedHead.setAttribute("aria-valuenow", latestPercentage.toString());
                DOM.algoIndexerCachedPercent.textContent = cachedPercentage.toString() + "%";
                DOM.algoIndexerBlocksIndexed.textContent = tailToHeadRange.toLocaleString();
                DOM.algoIndexerBlocksTotal.textContent = totalRange.toLocaleString();
                if (cachedPercentage === 100) {
                    DOM.algoIndexerProgressCached.classList.remove("progress-bar-striped", "progress-bar-animated");
                    DOM.algoIndexerProgressCached.classList.add("bg-success");
                    DOM.algoIndexerCachedPercent.classList.add("indexerComplete");
                } else {
                    DOM.algoIndexerProgressCached.classList.add("progress-bar-striped", "progress-bar-animated");
                    DOM.algoIndexerProgressCached.classList.remove("bg-success");
                    DOM.algoIndexerCachedPercent.classList.remove("indexerComplete");
                }
            } else {
                console.error("Failed to get Algorand indexer progress");
            }
        }
        async function getUploadDirectory() {
            let response = await HttpGetJson("/settings/uploadDirectory");
            if (response[0] === 200) {
                DOM.uploadDirectory.value = DOMPurify.sanitize(response[1].uploadDirectory);
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
        async function getIndexerRunning() {
            let response = await HttpGetJson("/settings/indexer/running");
            if (response[0] === 200) {
                DOM.indexerRunCheckbox.checked = response[1].indexerRunning;
            }
        }
        async function getBaseIndexerRunning() {
            let response = await HttpGetJson("/settings/base/indexer/running");
            if (response[0] === 200) {
                DOM.baseIndexerRunCheckbox.checked = response[1].indexerRunning;
            }
        }
        async function getAlgoIndexerRunning() {
            let response = await HttpGetJson("/settings/algorand/indexer/running");
            if (response[0] === 200) {
                DOM.algoIndexerRunCheckbox.checked = response[1].indexerRunning;
            }
        }
        async function getIndexerStatus() {
            let response = await HttpGetJson("/settings/indexer/status");
            if (response[0] === 200) {
                let status = DOMPurify.sanitize(response[1].status);
                let newText = "";
                let newColor = "";
                if (status == "running") {
                    newText = "Running";
                    newColor = "green";
                } else if (status == "complete") {
                    newText = "Complete";
                    newColor = "green";
                } else if (status == "stopped") {
                    newText = "Stopped";
                    newColor = "#D3D3D3";
                } else if (status == "failed") {
                    newText = "Failed";
                    newColor = "red";
                } else {
                    newText = status;
                    newColor = "yellow";
                }
                if (DOM.indexerStatusText.textContent !== newText) {
                    DOM.indexerStatusText.textContent = newText;
                }
                if (DOM.indexerStatusText.style.color !== newColor) {
                    DOM.indexerStatusText.style.color = newColor;
                }
            } else {
                LogError("Indexer Status Error");
            }
        }
        async function getBaseIndexerStatus() {
            let response = await HttpGetJson("/settings/base/indexer/status");
            if (response[0] === 200) {
                let status = DOMPurify.sanitize(response[1].status);
                updateIndexerStatusLight(DOM.baseIndexerStatusLight, status, "Base");
            }
        }
        async function getAlgoIndexerStatus() {
            let response = await HttpGetJson("/settings/algorand/indexer/status");
            if (response[0] === 200) {
                let status = DOMPurify.sanitize(response[1].status);
                updateIndexerStatusLight(DOM.algoIndexerStatusLight, status, "Algorand");
            }
        }
        function updateIndexerStatusLight(element: HTMLDivElement, status: string, blockchain: string) {
            let isActive = (status === "running" || status === "complete" || status === "pending");
            let tooltip = element.getAttribute("data-bs-original-title") || element.getAttribute("data-bs-title");
            let newTooltip = blockchain + " indexer " + (isActive ? "running" : "stopped");
            if (isActive) {
                element.classList.remove("redLight");
                element.classList.add("greenLight");
            } else {
                element.classList.remove("greenLight");
                element.classList.add("redLight");
            }
            if (tooltip !== newTooltip) {
                let bsTooltip = window.bootstrap.Tooltip.getInstance(element);
                if (bsTooltip) {
                    bsTooltip.setContent({".tooltip-inner": newTooltip});
                }
                element.setAttribute("data-bs-title", newTooltip);
                element.setAttribute("data-bs-original-title", newTooltip);
            }
        }
        async function getNetworkPorts() {
            DOM.retestPortsBtn.textContent = "";
            DOM.retestPortsBtn.classList.add("spinner-border");
            DOM.ipfsTrafficLight.classList.remove("greenLight");
            DOM.ipfsTrafficLight.classList.remove("redLight");
            DOM.yourplaceTrafficLight.classList.remove("greenLight");
            DOM.yourplaceTrafficLight.classList.remove("redLight");
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
            DOM.retestPortsBtn.classList.remove("spinner-border");
            DOM.retestPortsBtn.textContent = "Retest Ports";
        }
        async function getIpfsPinning() {
            let response = await HttpGetJson("/settings/content/ipfsPinning");
            if (response[0] === 200) {
                DOM.ipfsPinningURL.value = response[1].pinningURL || "";
                if (response[1].pinningURL && response[1].pinningURL !== "" && response[1].pinningKey && response[1].pinningKey !== "") {
                    DOM.ipfsPinningKey.value = "**********";
                } else {
                    DOM.ipfsPinningKey.value = "";
                }
            } else {
                ShowDialogModal("Failed to get IPFS Pinning settings");
            }
        }
        async function getIpfsGateway() {
            let response = await HttpGetJson("/settings/content/ipfsGateway");
            if (response[0] === 200) {
                DOM.ipfsGatewayURL.value = response[1].gateway || "ipfs.io";
            }
        }
        async function getDebugMode() {
            let response = await HttpGetJson("/settings/server/debug");
            DOM.serverDebugModeCheckbox.checked = false;
            if (response[0] === 200) {
                if (response[1].debug) {
                    DOM.serverDebugModeCheckbox.checked = true;
                }
            } else {
                ShowDialogModal("Failed to get server debug mode");
            }
        }
        async function getServerUpdates() {
            let serverVersion = await getServerVersion();
            let response = await HttpGetJson("https://yourplace.network/version");
            if (response[0] === 200) {
                let latestVersion = response[1].version;
                LogInfo("Latest YourPlace version: " + latestVersion);
                LogInfo("Current YourPlace version: " + serverVersion);
                if (serverVersion !== latestVersion) {
                    DisableDialogModalOkBtn();
                    ShowDialogModalHTML("A newer YourPlace version is available. Click <a href='https://yourplace.network/download' target='_blank'>here to update</a>");
                    EnableDialogModalOkBtn();
                } else {
                    ShowDialogModal("Your server is up to date!");
                }
            } else {
                ShowDialogModal("Failed to get server version");
            }
        }
        async function getServerVersion(): Promise<string> {
            let response = await HttpGetJson("/settings/server/version");
            if (response[0] === 200) {
                DOM.serverVersionText.textContent = response[1].version;
                DOM.helperVersionText.textContent = response[1].helperVersion;
                return response[1].version;
            } else {
                ShowDialogModal("Failed to get server version");
                return "";
            }
        }
        async function getServerLogs() {
            DOM.logsView.textContent = "";
            let response = await HttpGetJson("/settings/server/logs/view");
            if (response[0] === 200) {
                //ShowDialogModalHTML(response[1].logs);
                DOM.logsView.textContent = response[1].logs;
                DOM.logsView.classList.remove("hidden");
            } else {
                ShowDialogModal("Failed to get server logs");
            }
        }
        async function getHelperLogs() {
            DOM.logsView.textContent = "";
            let response = await HttpGetJson("/settings/helper/logs/view");
            if (response[0] === 200) {
                //ShowDialogModalHTML(response[1].logs);
                DOM.logsView.textContent = response[1].logs;
                DOM.logsView.classList.remove("hidden");
            } else {
                ShowDialogModal("Failed to get server logs");
            }
        }
        async function getServerRuntime() {
            let response = await HttpGetJson("/settings/server/runtime");
            if (response[0] === 200) {
                let flags: string[] = [];
                if (response[1].flags) {
                    if (response[1].flags.debug) flags.push("-d (debug)");
                    if (response[1].flags.gateway) flags.push("-g (gateway)");
                }
                DOM.runtimeFlagsText.textContent = flags.length > 0 ? flags.join(", ") : "None";
                let envVarsHTML: string[] = [];
                if (response[1].envVars && Object.keys(response[1].envVars).length > 0) {
                    for (const [key, value] of Object.entries(response[1].envVars)) {
                        envVarsHTML.push(`<div><code>${DOMPurify.sanitize(key)}</code>: ${DOMPurify.sanitize(value as string)}</div>`);
                    }
                    DOM.runtimeEnvVarsText.innerHTML = envVarsHTML.join("");
                } else {
                    DOM.runtimeEnvVarsText.textContent = "None";
                }
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
                await getBaseThrottle();
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
                DOM.baseThrottle.value = response[1].defaultBaseThrottle;
                DOM.baseThrottleNumber.textContent = response[1].defaultBaseThrottle;
                DOM.baseThrottle.disabled = true;
                ShowSavedToast();
            }
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
        async function setBaseIndexerReset() {
            const confirmed = await ShowModalYesNoHTML("⚠️ Are you sure you want to reset the indexer? ⚠️<br><br>This will delete cached YourPlace data and re-index everything<br><br>It will take a long time and download a lot of data<br><br>Your personal data, posts, and profile <u>will not</u> be deleted");
            if (confirmed) {
                let response = await HttpPostJson("/settings/base/indexerReset",
                    {indexerReset: true}, DOM.csrfToken.value);
                if (response[0] === 200) {
                    LogInfo("Base Indexer Reset");
                    ShowSavedToast();
                } else {
                    LogInfo("Base Indexer Reset Error");
                }
            }
        }
        async function setBaseIndexerCatchUp(variable: string) {
            switch (variable) {
                case "full":
                    let response = await HttpPostJson("/settings/base/indexerCatchUp",
                        {indexerCatchUp: "full"}, DOM.csrfToken.value);
                    if (response[0] === 200) {
                        LogInfo("Base Indexer Full Catch-Up Started");
                        ShowSavedToast();
                    } else {
                        ShowDialogModal(response[1].status);
                    }
                    break;
                case "h":
                    ShowDialogModalHTML("This Indexer Catch-Up feature will download a fully cached copy of YourPlace data that we've already downloaded from the blockchain. This prevents you from needing to traverse the whole chain, and allows Servers to quickly catch up to the latest data.<br><br>This will save you bandwidth and time, but can only be run once every 24 hours.<br><br>To stream YourPlace data in real-time, you will still need your own blockchain RPC server.");
                    break;
            }
        }
        // Algorand Setters
        async function setAlgoURL() {
            const data = {
                algoURL: DOM.algoURL.value,
            }
            let response = await HttpPostJson("/settings/algorand/url", data, DOM.csrfToken.value);
            if (response[0] === 200) {
                LogInfo("Algorand URL Saved");
                ShowSavedToast();
                await getAlgoThrottle();
            } else {
                DOM.algoURL.value = DOMPurify.sanitize(response[1].status);
            }
        }
        async function setAlgoThrottle() {
            const data = {
                throttle: DOM.algoThrottle.valueAsNumber,
            }
            let response = await HttpPostJson("/settings/algorand/throttle", data, DOM.csrfToken.value);
            if (response[0] === 200) {
                LogInfo("Algorand Throttle Saved");
                DOM.algoThrottleNumber.textContent = DOM.algoThrottle.value;
                ShowSavedToast();
            }
        }
        async function setDefaultAlgoURL() {
            const data = {
                algoURL: "default",
            }
            let response = await HttpPostJson("/settings/algorand/url", data, DOM.csrfToken.value);
            if (response[0] === 200) {
                DOM.algoURL.value = response[1].defaultAlgoURL;
                DOM.algoThrottle.value = response[1].defaultAlgoThrottle;
                DOM.algoThrottleNumber.textContent = response[1].defaultAlgoThrottle;
                DOM.algoThrottle.disabled = true;
                ShowSavedToast();
            }
        }
        async function setAlgoIndexerReset() {
            const confirmed = await ShowModalYesNoHTML("⚠️ Are you sure you want to reset the Algorand indexer? ⚠️<br><br>This will delete cached Algorand YourPlace data and re-index everything<br><br>It will take a long time and download a lot of data<br><br>Your personal data, posts, and profile <u>will not</u> be deleted");
            if (confirmed) {
                let response = await HttpPostJson("/settings/algorand/indexerReset",
                    {indexerReset: true}, DOM.csrfToken.value);
                if (response[0] === 200) {
                    LogInfo("Algorand Indexer Reset");
                    ShowSavedToast();
                } else {
                    LogInfo("Algorand Indexer Reset Error");
                }
            }
        }
        async function setAlgoIndexerCatchUp(variable: string) {
            switch (variable) {
                case "full":
                    let response = await HttpPostJson("/settings/algorand/indexerCatchUp",
                        {indexerCatchUp: "full"}, DOM.csrfToken.value);
                    if (response[0] === 200) {
                        LogInfo("Algorand Indexer Full Catch-Up Started");
                        ShowSavedToast();
                    } else {
                        ShowDialogModal(response[1].status);
                    }
                    break;
                case "h":
                    ShowDialogModalHTML("This Indexer Catch-Up feature will download a fully cached copy of YourPlace data that we've already downloaded from the blockchain. This prevents you from needing to traverse the whole chain, and allows Servers to quickly catch up to the latest data.<br><br>This will save you bandwidth and time, but can only be run once every 24 hours.<br><br>To stream YourPlace data in real-time, you will still need your own blockchain RPC server.");
                    break;
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
        async function setIndexerRunning() {
            if (DOM.indexerRunCheckbox.checked) {
                let response = await HttpPostJson("/settings/indexer/start", {indexerRun: DOM.indexerRunCheckbox.checked}, DOM.csrfToken.value);
                if (response[0] === 200) {
                    LogInfo(response[1].status);
                    DOM.indexerOnBatteryCheckbox.disabled = false;
                    DOM.baseIndexerRunCheckbox.checked = true;
                    DOM.algoIndexerRunCheckbox.checked = true;
                } else {
                    LogError("Indexer Run Error");
                }
            } else {
                let response = await HttpPostJson("/settings/indexer/stop", {indexerRun: DOM.indexerRunCheckbox.checked}, DOM.csrfToken.value);
                if (response[0] === 200) {
                    LogInfo(response[1].status);
                    DOM.indexerOnBatteryCheckbox.disabled = true;
                    DOM.baseIndexerRunCheckbox.checked = false;
                    DOM.algoIndexerRunCheckbox.checked = false;
                } else {
                    LogError("Indexer Stop Error");
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
        async function setBaseIndexerRunning() {
            let response = await HttpPostJson("/settings/base/indexer/running",
                {indexerRunning: DOM.baseIndexerRunCheckbox.checked}, DOM.csrfToken.value);
            if (response[0] === 200) {
                LogInfo("Base Indexer " + (DOM.baseIndexerRunCheckbox.checked ? "Started" : "Stopped"));
                ShowSavedToast();
                updateGlobalIndexerCheckbox();
            } else {
                LogError("Base Indexer Toggle Error");
                DOM.baseIndexerRunCheckbox.checked = !DOM.baseIndexerRunCheckbox.checked;
            }
        }
        async function setAlgoIndexerRunning() {
            let response = await HttpPostJson("/settings/algorand/indexer/running",
                {indexerRunning: DOM.algoIndexerRunCheckbox.checked}, DOM.csrfToken.value);
            if (response[0] === 200) {
                LogInfo("Algorand Indexer " + (DOM.algoIndexerRunCheckbox.checked ? "Started" : "Stopped"));
                ShowSavedToast();
                updateGlobalIndexerCheckbox();
            } else {
                LogError("Algorand Indexer Toggle Error");
                DOM.algoIndexerRunCheckbox.checked = !DOM.algoIndexerRunCheckbox.checked;
            }
        }
        function updateGlobalIndexerCheckbox() {
            const anyRunning = DOM.baseIndexerRunCheckbox.checked || DOM.algoIndexerRunCheckbox.checked;
            DOM.indexerRunCheckbox.checked = anyRunning;
        }
        async function setIPFSPinning() {
            if (DOM.ipfsPinningKey.value === "**********") {
                ShowDialogModal("Please enter a new pinning key or clear the field to remove the pinning service");
                return;
            }
            const data = {
                pinningURL: DOM.ipfsPinningURL.value,
                pinningKey: DOM.ipfsPinningKey.value,
            }
            let response = await HttpPostJson("/settings/content/ipfsPinning", data, DOM.csrfToken.value);
            if (response[0] === 200) {
                ShowSavedToast();
                await getIpfsPinning();
            } else {
                ShowDialogModal(response[1].status || "Failed to save IPFS pinning settings");
            }
        }
        async function setRemoveIPFSPinning() {
            let response = await HttpPostJson("/settings/content/ipfsPinning/remove", {}, DOM.csrfToken.value);
            if (response[0] === 200) {
                DOM.ipfsPinningURL.value = "";
                DOM.ipfsPinningKey.value = "";
                ShowSavedToast();
            }
        }
        async function setIpfsGateway() {
            const data = {
                gateway: DOM.ipfsGatewayURL.value,
            }
            let response = await HttpPostJson("/settings/content/ipfsGateway", data, DOM.csrfToken.value);
            if (response[0] === 200) {
                ShowSavedToast();
            } else {
                ShowDialogModal(response[1].status || "Failed to save IPFS gateway");
            }
        }
        async function setDefaultIpfsGateway() {
            const data = {
                gateway: "default",
            }
            let response = await HttpPostJson("/settings/content/ipfsGateway", data, DOM.csrfToken.value);
            if (response[0] === 200) {
                DOM.ipfsGatewayURL.value = response[1].gateway || "ipfs.io";
                ShowSavedToast();
            }
        }
        async function setDebugMode() {
            const data = {
                debug: DOM.serverDebugModeCheckbox.checked,
            }
            let response = await HttpPostJson("/settings/server/debug", data, DOM.csrfToken.value);
            ShowSavedToast(); // todo: need a more graceful exit, as the server may restart at this point
        }
        async function setUninstall() {
            const uninstallHTML = "<span id=\"uninstallTitle\">Do you want to uninstall YourPlace? 🗑️</span><br>" +
                "<div class=\"form-check form-switch uninstallCheck\">\n" +
                "  <input class=\"form-check-input\" type=\"checkbox\" role=\"switch\" id=\"keepUploadFilesCheck\" checked>\n" +
                "  <label class=\"form-check-label\" for=\"switchCheckDefault\">Keep Uploaded Files</label>\n" +
                "</div>" +
                "<div class=\"form-check form-switch uninstallCheck\">\n" +
                "  <input class=\"form-check-input\" type=\"checkbox\" role=\"switch\" id=\"keepBlockchainDataCheck\">\n" +
                "  <label class=\"form-check-label\" for=\"switchCheckDefault\">Keep Blockchain Data</label>\n" +
                "</div>";
            const confirmed = await ShowModalYesNoHTML(uninstallHTML);
            if (confirmed) {
                let keepUploadFiles = (document.getElementById("keepUploadFilesCheck") as HTMLInputElement).checked;
                let keepBlockchainData = (document.getElementById("keepBlockchainDataCheck") as HTMLInputElement).checked;
                let data = {
                    uploadFiles: keepUploadFiles,
                    blockchainData: keepBlockchainData,
                }
                let response = await HttpPostJson("/settings/uninstall", data, DOM.csrfToken.value);
                if (response[0] === 200) {
                    ShowDialogModal("Uninstalling YourPlace...\nPlease Wait");
                    await Sleep(10000);
                    window.location.href = "https://yourplace.network/uninstall";
                } else {
                    ShowDialogModal(response[1].message);
                }
            }
        }
        async function setTorHiddenService() {
            const data = {
                enabled: DOM.torHiddenServiceCheck.checked,
            }
            let response = await HttpPostJson("/settings/privacy/torHiddenService", data, DOM.csrfToken.value);
            if (response[0] === 200 && response[1].status === "success") {
                ShowSavedToast();
            } else {
                DOM.torHiddenServiceCheck.checked = !DOM.torHiddenServiceCheck.checked;
                ShowDialogModal(response[1].status || "Failed to toggle TOR hidden service");
            }
        }
        let xcomCredentialsValid = false;
        let xcomIsFreeTier = true;
        let xcomRateLimited = false;
        async function getXcomSettings() {
            let response = await HttpGetJson("/settings/services/xcom/settings");
            if (response[0] === 200) {
                DOM.xcomCrossPostCheckbox.checked = response[1].crossPostEnabled;
                DOM.xcomFeedAggregationCheckbox.checked = response[1].feedAggregationEnabled;
                updateXcomScrapeCredentialsVisibility();
            }
        }
        async function getXcomTier() {
            let response = await HttpGetJson("/settings/services/xcom/tier");
            if (response[0] === 200) {
                xcomIsFreeTier = response[1].isFreeTier;
                updateXcomScrapeCredentialsVisibility();
            }
        }
        function updateXcomScrapeCredentialsVisibility() {
            if (xcomIsFreeTier && DOM.xcomFeedAggregationCheckbox.checked) {
                DOM.xcomScrapeCredentialsDiv.style.display = "block";
            } else {
                DOM.xcomScrapeCredentialsDiv.style.display = "none";
            }
        }
        function showXcomCredentialsModal() {
            ShowDialogModalHTML(
                "<b>X.com API Credentials Required</b><br><br>" +
                "To use X.com features, you need to create an X.com Developer account and generate API credentials.<br><br>" +
                "<b>Steps:</b><br>" +
                "1. Go to the <a href='https://developer.x.com/en/portal/dashboard' target='_blank'>X Developer Portal</a><br>" +
                "2. Create a new App or use an existing one<br>" +
                "3. Generate your API Key, API Secret, Access Token, and Access Token Secret<br>" +
                "4. Enter the credentials above and click Save<br><br>" +
                "<a href='https://developer.x.com/en/docs/authentication/oauth-1-0a' target='_blank'>View X.com OAuth Documentation</a>"
            );
        }
        async function setXcomCrossPost() {
            const enabling = DOM.xcomCrossPostCheckbox.checked;
            if (enabling) {
                await getXcomCredentials();
                if (!xcomCredentialsValid) {
                    DOM.xcomCrossPostCheckbox.checked = false;
                    showXcomCredentialsModal();
                    return;
                }
            }
            let response = await HttpPostJson("/settings/services/xcom/crosspost", {enabled: enabling}, DOM.csrfToken.value);
            if (response[0] === 200) {
                ShowSavedToast();
            } else {
                DOM.xcomCrossPostCheckbox.checked = !enabling;
                ShowDialogModal(response[1].status || "Failed to update cross-post setting");
            }
        }
        async function setXcomFeedAggregation() {
            const enabling = DOM.xcomFeedAggregationCheckbox.checked;
            if (enabling) {
                await getXcomCredentials();
                if (!xcomCredentialsValid) {
                    DOM.xcomFeedAggregationCheckbox.checked = false;
                    showXcomCredentialsModal();
                    return;
                }
            }
            if (enabling) {
                await getXcomTier();
                if (xcomIsFreeTier) {
                    let response = await HttpPostJson("/settings/services/xcom/feedaggregation", {enabled: true}, DOM.csrfToken.value);
                    if (response[0] === 200) {
                        ShowSavedToast();
                        updateXcomScrapeCredentialsVisibility();
                    } else {
                        DOM.xcomFeedAggregationCheckbox.checked = false;
                        ShowDialogModal(response[1].status || "Failed to enable feed aggregation");
                    }
                } else {
                    let response = await HttpPostJson("/settings/services/xcom/feedaggregation", {enabled: true}, DOM.csrfToken.value);
                    if (response[0] === 200) {
                        ShowSavedToast();
                        updateXcomScrapeCredentialsVisibility();
                    } else {
                        DOM.xcomFeedAggregationCheckbox.checked = false;
                        ShowDialogModal(response[1].status || "Failed to enable feed aggregation");
                    }
                }
            } else {
                let response = await HttpPostJson("/settings/services/xcom/feedaggregation", {enabled: false}, DOM.csrfToken.value);
                if (response[0] === 200) {
                    ShowSavedToast();
                    updateXcomScrapeCredentialsVisibility();
                } else {
                    DOM.xcomFeedAggregationCheckbox.checked = true;
                    ShowDialogModal(response[1].status || "Failed to disable feed aggregation");
                }
            }
        }
        async function getXcomCredentials() {
            let response = await HttpGetJson("/settings/services/xcom/credentials");
            if (response[0] === 200) {
                DOM.xcomApiKey.value = response[1].apiKey || "";
                if (response[1].hasCredentials) {
                    DOM.xcomApiSecret.value = "**********";
                    DOM.xcomAccessToken.value = response[1].accessToken || "";
                    DOM.xcomAccessTokenSecret.value = "**********";
                } else {
                    DOM.xcomApiSecret.value = "";
                    DOM.xcomAccessToken.value = "";
                    DOM.xcomAccessTokenSecret.value = "";
                }
                xcomCredentialsValid = response[1].isValid;
                xcomRateLimited = response[1].rateLimited || false;
                updateXcomStatusLight(response[1].isValid);
                updateXcomRateLimitUI(response[1].rateLimited, response[1].rateLimitRemaining);
            }
        }
        function updateXcomRateLimitUI(rateLimited: boolean, remaining: string) {
            if (rateLimited) {
                DOM.testXcomCredentialsBtn.disabled = true;
                DOM.testXcomCredentialsBtn.textContent = "Rate Limited";
                DOM.testXcomCredentialsBtn.title = "X.com API rate limited. " + remaining + " remaining.";
            } else {
                DOM.testXcomCredentialsBtn.disabled = false;
                DOM.testXcomCredentialsBtn.textContent = "Test";
                DOM.testXcomCredentialsBtn.title = "";
            }
        }
        async function setXcomCredentials() {
            if (DOM.xcomApiSecret.value === "**********" || DOM.xcomAccessTokenSecret.value === "**********") {
                ShowDialogModal("Please enter new credentials or clear the fields to remove them");
                return;
            }
            const data = {
                apiKey: DOM.xcomApiKey.value,
                apiSecret: DOM.xcomApiSecret.value,
                accessToken: DOM.xcomAccessToken.value,
                accessTokenSecret: DOM.xcomAccessTokenSecret.value,
            }
            let response = await HttpPostJson("/settings/services/xcom/credentials", data, DOM.csrfToken.value);
            if (response[0] === 429 || response[1].rateLimited) {
                xcomRateLimited = true;
                updateXcomRateLimitUI(true, "24h0m0s");
                ShowDialogModalHTML(
                    "<b>X.com API Rate Limited</b><br><br>" +
                    "You have exceeded X.com's API rate limits. Credential testing is disabled for 24 hours.<br><br>" +
                    "Please try saving your credentials again later."
                );
                return;
            }
            if (response[0] === 200) {
                ShowSavedToast();
                await getXcomCredentials();
                await getXcomTier();
                updateXcomScrapeCredentialsVisibility();
            } else {
                ShowDialogModal(response[1].status || "Failed to save X.com credentials");
            }
        }
        async function removeXcomCredentials() {
            let response = await HttpPostJson("/settings/services/xcom/credentials/remove", {}, DOM.csrfToken.value);
            if (response[0] === 200) {
                DOM.xcomApiKey.value = "";
                DOM.xcomApiSecret.value = "";
                DOM.xcomAccessToken.value = "";
                DOM.xcomAccessTokenSecret.value = "";
                xcomCredentialsValid = false;
                xcomIsFreeTier = true;
                updateXcomStatusLight(false);
                updateXcomScrapeCredentialsVisibility();
                ShowSavedToast();
            } else {
                ShowDialogModal(response[1].status || "Failed to remove X.com credentials");
            }
        }
        async function testXcomCredentials() {
            if (xcomRateLimited) {
                ShowDialogModalHTML(
                    "<b>X.com API Rate Limited</b><br><br>" +
                    "You have exceeded X.com's API rate limits. Testing is disabled for 24 hours to prevent further issues.<br><br>" +
                    "Your credentials are saved and will continue to work once the rate limit expires."
                );
                return;
            }
            DOM.testXcomCredentialsBtn.disabled = true;
            DOM.testXcomCredentialsBtn.textContent = "Testing...";
            let response = await HttpGetJson("/settings/services/xcom/test");
            if (response[1].rateLimited) {
                xcomRateLimited = true;
                updateXcomRateLimitUI(true, "24h0m0s");
                ShowDialogModalHTML(
                    "<b>X.com API Rate Limited</b><br><br>" +
                    "You have exceeded X.com's API rate limits. Testing is disabled for 24 hours to prevent further issues.<br><br>" +
                    "Your credentials are saved and will continue to work once the rate limit expires."
                );
                return;
            }
            DOM.testXcomCredentialsBtn.disabled = false;
            DOM.testXcomCredentialsBtn.textContent = "Test";
            if (response[0] === 200 && response[1].isValid) {
                updateXcomStatusLight(true);
                ShowToast("X.com credentials are valid");
            } else {
                updateXcomStatusLight(false);
                ShowDialogModal(response[1].status || "X.com credentials are invalid");
            }
        }
        function updateXcomStatusLight(isValid: boolean) {
            let tooltip = DOM.xcomStatusLight.getAttribute("data-bs-original-title") || DOM.xcomStatusLight.getAttribute("data-bs-title");
            let newTooltip = isValid ? "API credentials valid" : "API credentials not configured";
            if (isValid) {
                DOM.xcomStatusLight.classList.remove("redLight");
                DOM.xcomStatusLight.classList.add("greenLight");
            } else {
                DOM.xcomStatusLight.classList.remove("greenLight");
                DOM.xcomStatusLight.classList.add("redLight");
            }
            if (tooltip !== newTooltip) {
                let bsTooltip = window.bootstrap.Tooltip.getInstance(DOM.xcomStatusLight);
                if (bsTooltip) {
                    bsTooltip.setContent({".tooltip-inner": newTooltip});
                }
                DOM.xcomStatusLight.setAttribute("data-bs-title", newTooltip);
                DOM.xcomStatusLight.setAttribute("data-bs-original-title", newTooltip);
            }
        }
        function updateXcomScrapeStatusLight(isValid: boolean) {
            let tooltip = DOM.xcomScrapeStatusLight.getAttribute("data-bs-original-title") || DOM.xcomScrapeStatusLight.getAttribute("data-bs-title");
            let newTooltip = isValid ? "Login credentials valid" : "Login credentials not configured";
            if (isValid) {
                DOM.xcomScrapeStatusLight.classList.remove("redLight");
                DOM.xcomScrapeStatusLight.classList.add("greenLight");
            } else {
                DOM.xcomScrapeStatusLight.classList.remove("greenLight");
                DOM.xcomScrapeStatusLight.classList.add("redLight");
            }
            if (tooltip !== newTooltip) {
                let bsTooltip = window.bootstrap.Tooltip.getInstance(DOM.xcomScrapeStatusLight);
                if (bsTooltip) {
                    bsTooltip.setContent({".tooltip-inner": newTooltip});
                }
                DOM.xcomScrapeStatusLight.setAttribute("data-bs-title", newTooltip);
                DOM.xcomScrapeStatusLight.setAttribute("data-bs-original-title", newTooltip);
            }
        }
        async function getXcomScrapeCredentials() {
            let response = await HttpGetJson("/settings/services/xcom/scrape/credentials");
            if (response[0] === 200) {
                DOM.xcomScrapeEmail.value = response[1].email || "";
                DOM.xcomScrapeUsername.value = response[1].username || "";
                if (response[1].hasPassword) {
                    DOM.xcomScrapePassword.value = "**********";
                } else {
                    DOM.xcomScrapePassword.value = "";
                }
                updateXcomScrapeStatusLight(response[1].isValid || false);
            }
        }
        async function setXcomScrapeCredentials() {
            if (DOM.xcomScrapePassword.value === "**********") {
                ShowDialogModal("Please enter a new password or clear the field to remove credentials");
                return;
            }
            let confirmed = await ShowModalYesNoHTML(
                "<b>Experimental Feature Warning</b><br><br>" +
                "This screen scraping feature is <b>experimental</b> and may cause issues with your X.com account, " +
                "including but not limited to account suspension or termination.<br><br>" +
                "We recommend purchasing <a href='https://developer.x.com/en/portal/products' target='_blank'>Basic or Pro API access</a> " +
                "for reliable feed aggregation.<br><br>" +
                "By proceeding, you accept all responsibility for its use.<br><br>" +
                "Do you wish to continue?"
            );
            if (!confirmed) {
                await removeXcomScrapeCredentials();
                DOM.xcomFeedAggregationCheckbox.checked = false;
                await HttpPostJson("/settings/services/xcom/feedaggregation", {enabled: false}, DOM.csrfToken.value);
                updateXcomScrapeCredentialsVisibility();
                return;
            }
            const data = {
                email: DOM.xcomScrapeEmail.value,
                username: DOM.xcomScrapeUsername.value,
                password: DOM.xcomScrapePassword.value,
            }
            let response = await HttpPostJson("/settings/services/xcom/scrape/credentials", data, DOM.csrfToken.value);
            if (response[0] === 200) {
                ShowSavedToast();
                await getXcomScrapeCredentials();
            } else {
                ShowDialogModal(response[1].status || "Failed to save X.com scraping credentials");
            }
        }
        async function removeXcomScrapeCredentials() {
            let response = await HttpPostJson("/settings/services/xcom/scrape/credentials/remove", {}, DOM.csrfToken.value);
            if (response[0] === 200) {
                DOM.xcomScrapeEmail.value = "";
                DOM.xcomScrapeUsername.value = "";
                DOM.xcomScrapePassword.value = "";
                updateXcomScrapeStatusLight(false);
                ShowSavedToast();
            } else {
                ShowDialogModal(response[1].status || "Failed to remove X.com scraping credentials");
            }
        }
        async function testXcomScrapeCredentials() {
            DOM.testXcomScrapeCredentialsBtn.disabled = true;
            DOM.testXcomScrapeCredentialsBtn.textContent = "Testing...";
            let response = await HttpGetJson("/settings/services/xcom/scrape/test");
            DOM.testXcomScrapeCredentialsBtn.disabled = false;
            DOM.testXcomScrapeCredentialsBtn.textContent = "Test";
            if (response[0] === 200 && response[1].isValid) {
                updateXcomScrapeStatusLight(true);
                ShowToast("Login credentials are valid");
            } else {
                updateXcomScrapeStatusLight(false);
                ShowDialogModal(response[1].status || "Login credentials are invalid");
            }
        }

        /* Throttle Slider */
        function getThumbElement(slider: HTMLInputElement): HTMLElement {
            const thumbPosition = (Number(slider.value) - Number(slider.min)) /
                (Number(slider.max) - Number(slider.min));
            const inputRect = slider.getBoundingClientRect();
            const thumbWidth = 16;
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
                    toJSON: () => {}
                }),
                clientWidth: thumbWidth,
                clientHeight: thumbWidth
            } as unknown as HTMLElement;
        }
        function showBaseTooltip() {
            DOM.baseThrottleTooltip.style.display = 'block';
            if (!popperInstance) {
                popperInstance = createPopper(getThumbElement(DOM.baseThrottle), DOM.baseThrottleTooltip, {
                    placement: 'top',
                    modifiers: [{name: 'offset', options: {offset: [0, 8]}}],
                });
            }
        }
        function hideBaseTooltip() {
            DOM.baseThrottleTooltip.style.display = 'none';
            if (popperInstance) {
                popperInstance.destroy();
                popperInstance = null;
            }
        }
        function updateBaseTooltip() {
            DOM.baseThrottleTooltip.querySelector('.throttle-tooltip-inner')!.textContent = DOM.baseThrottle.value;
            if (popperInstance) {
                popperInstance.state.elements.reference = getThumbElement(DOM.baseThrottle);
                popperInstance.update();
            }
        }
        function showAlgoTooltip() {
            DOM.algoThrottleTooltip.style.display = 'block';
            if (!algoPopperInstance) {
                algoPopperInstance = createPopper(getThumbElement(DOM.algoThrottle), DOM.algoThrottleTooltip, {
                    placement: 'top',
                    modifiers: [{name: 'offset', options: {offset: [0, 8]}}],
                });
            }
        }
        function hideAlgoTooltip() {
            DOM.algoThrottleTooltip.style.display = 'none';
            if (algoPopperInstance) {
                algoPopperInstance.destroy();
                algoPopperInstance = null;
            }
        }
        function updateAlgoTooltip() {
            DOM.algoThrottleTooltip.querySelector('.throttle-tooltip-inner')!.textContent = DOM.algoThrottle.value;
            if (algoPopperInstance) {
                algoPopperInstance.state.elements.reference = getThumbElement(DOM.algoThrottle);
                algoPopperInstance.update();
            }
        }
        DOM.baseThrottle.addEventListener("input", () => { showBaseTooltip(); updateBaseTooltip(); });
        DOM.baseThrottle.addEventListener("mousedown", showBaseTooltip);
        DOM.baseThrottle.addEventListener("touchstart", showBaseTooltip);
        DOM.baseThrottle.addEventListener("mouseup", hideBaseTooltip);
        DOM.baseThrottle.addEventListener("mouseleave", hideBaseTooltip);
        DOM.baseThrottle.addEventListener("touchend", hideBaseTooltip);
        DOM.algoThrottle.addEventListener("input", () => { showAlgoTooltip(); updateAlgoTooltip(); });
        DOM.algoThrottle.addEventListener("mousedown", showAlgoTooltip);
        DOM.algoThrottle.addEventListener("touchstart", showAlgoTooltip);
        DOM.algoThrottle.addEventListener("mouseup", hideAlgoTooltip);
        DOM.algoThrottle.addEventListener("mouseleave", hideAlgoTooltip);
        DOM.algoThrottle.addEventListener("touchend", hideAlgoTooltip);

        /* Event Listeners */
        DOM.baseDataDirectory!.addEventListener("change", setBaseDataDirectory);
        DOM.baseDefaultDataDirectoryBtn!.addEventListener("click", setDefaultBaseDataDirectory);
        DOM.baseFullNodeCheckbox!.addEventListener("change", setBaseFullNode);
        DOM.baseThrottle!.addEventListener("change", setBaseThrottle);
        DOM.baseSaveDataDirectoryBtn!.addEventListener("click", setBaseDataDirectory);
        DOM.baseIndexerResetBtn!.addEventListener("click", setBaseIndexerReset);
        DOM.baseCatchUpFullBtn!.addEventListener("click", function() { setBaseIndexerCatchUp("full").then(); });
        DOM.baseCatchUpHelpBtn!.addEventListener("click", function() { setBaseIndexerCatchUp("h").then(); });
        DOM.defaultBaseURLBtn!.addEventListener("click", setDefaultBaseURL);
        DOM.defaultUploadDirectoryBtn!.addEventListener("click", setDefaultUploadDirectory);
        DOM.saveBaseURLBtn!.addEventListener("click", setBaseURL);
        DOM.saveUploadDirectoryBtn!.addEventListener("click", setUploadDirectory);
        DOM.spiceometerCheck!.addEventListener("change", setSpiceometer);
        DOM.indexerRunCheckbox!.addEventListener("change", setIndexerRunning);
        DOM.indexerOnBatteryCheckbox!.addEventListener("change", setIndexerOnBattery);
        DOM.retestPortsBtn!.addEventListener("click", getNetworkPorts);
        DOM.pinataLI!.addEventListener("click", function(e) {
            DOM.ipfsPinningURL.value = "https://api.pinata.cloud/psa";
            DOM.ipfsPinningKey.value = "";
            ShowDialogModalHTML("Please create an account and an <b>API Key</b> from <a href='https://app.pinata.cloud/' target='_blank'>Pinata here</a><br><br>Then add your <b>JWT (secret access token)</b> to the IPFS Pinning settings page<br><br>Ensure your key has admin and write all privileges");
        });
        DOM.saveIpfsPinningBtn.addEventListener("click", setIPFSPinning);
        DOM.removeIpfsPinningBtn.addEventListener("click", setRemoveIPFSPinning);
        DOM.ipfsPinningKey.addEventListener("focus", function() {
            if (DOM.ipfsPinningKey.value === "**********") {
                DOM.ipfsPinningKey.value = "";
            }
        });
        DOM.saveIpfsGatewayBtn.addEventListener("click", setIpfsGateway);
        DOM.defaultIpfsGatewayBtn.addEventListener("click", setDefaultIpfsGateway);
        DOM.serverDebugModeCheckbox.addEventListener("change", setDebugMode);
        DOM.serverUpdateBtn.addEventListener("click", getServerUpdates);
        DOM.serverUninstallBtn.addEventListener("click", setUninstall);
        DOM.serverLogsViewBtn.addEventListener("click", getServerLogs);
        DOM.helperLogsViewBtn.addEventListener("click", getHelperLogs);
        DOM.torHiddenServiceCheck.addEventListener("change", setTorHiddenService);
        DOM.baseIndexerRunCheckbox!.addEventListener("change", setBaseIndexerRunning);
        DOM.algoThrottle!.addEventListener("change", setAlgoThrottle);
        DOM.algoIndexerResetBtn!.addEventListener("click", setAlgoIndexerReset);
        DOM.algoCatchUpFullBtn!.addEventListener("click", function() { setAlgoIndexerCatchUp("full").then(); });
        DOM.algoCatchUpHelpBtn!.addEventListener("click", function() { setAlgoIndexerCatchUp("h").then(); });
        DOM.defaultAlgoURLBtn!.addEventListener("click", setDefaultAlgoURL);
        DOM.saveAlgoURLBtn!.addEventListener("click", setAlgoURL);
        DOM.algoIndexerRunCheckbox!.addEventListener("change", setAlgoIndexerRunning);
        DOM.xcomCrossPostCheckbox.addEventListener("change", setXcomCrossPost);
        DOM.xcomFeedAggregationCheckbox.addEventListener("change", setXcomFeedAggregation);
        DOM.saveXcomCredentialsBtn.addEventListener("click", setXcomCredentials);
        DOM.removeXcomCredentialsBtn.addEventListener("click", removeXcomCredentials);
        DOM.testXcomCredentialsBtn.addEventListener("click", testXcomCredentials);
        DOM.xcomApiSecret.addEventListener("focus", function() {
            if (DOM.xcomApiSecret.value === "**********") {
                DOM.xcomApiSecret.value = "";
            }
        });
        DOM.xcomAccessTokenSecret.addEventListener("focus", function() {
            if (DOM.xcomAccessTokenSecret.value === "**********") {
                DOM.xcomAccessTokenSecret.value = "";
            }
        });
        DOM.saveXcomScrapeCredentialsBtn.addEventListener("click", setXcomScrapeCredentials);
        DOM.removeXcomScrapeCredentialsBtn.addEventListener("click", removeXcomScrapeCredentials);
        DOM.testXcomScrapeCredentialsBtn.addEventListener("click", testXcomScrapeCredentials);
        DOM.xcomScrapePassword.addEventListener("focus", function() {
            if (DOM.xcomScrapePassword.value === "**********") {
                DOM.xcomScrapePassword.value = "";
            }
        });

        /* On-Demand Loading */
        DOM.collapseContent.addEventListener("show.bs.collapse", function() {
            getUploadDirectory().then();
            getIpfsPinning().then();
            getIpfsGateway().then();
            getSpiceometer().then();
            getOllamaEnabled().then();
            getOllamaModelEnabled().then();
        });
        DOM.collapseBlockchain.addEventListener("show.bs.collapse", function() {
            getIndexerOnBattery().then();
            getIndexerRunning().then();
            getIndexerStatus().then();
        });
        DOM.collapseBase.addEventListener("show.bs.collapse", function() {
            getBaseURL().then();
            getBaseIndexerProgress().then();
            getBaseThrottle().then();
            getBaseFullNode().then();
            getBaseDataDirectory().then();
            getBaseIndexerRunning().then();
            getBaseIndexerStatus().then();
        });
        DOM.collapseAlgo.addEventListener("show.bs.collapse", function() {
            getAlgoURL().then();
            getAlgoIndexerProgress().then();
            getAlgoThrottle().then();
            getAlgoIndexerRunning().then();
            getAlgoIndexerStatus().then();
        });
        DOM.collapseServerInfo.addEventListener("show.bs.collapse", function() {
            getDebugMode().then();
            getServerRuntime().then();
            getServerVersion().then();
        });
        DOM.collapseNetworking.addEventListener("show.bs.collapse", function() {
            getNetworkPorts().then();
        });
        DOM.collapseXcom.addEventListener("show.bs.collapse", function() {
            getXcomCredentials().then();
            getXcomSettings().then();
            getXcomTier().then();
            getXcomScrapeCredentials().then();
        });

        init().then();
    }
})();