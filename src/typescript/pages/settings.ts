window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/global.scss";
import "../../scss/pages/settings.scss";
import "../components/menu";
import DOMPurify from "dompurify";
import {HttpGetJson, HttpPostJson} from "../util/network";
import {LogError, LogInfo} from "../util/log";
import {createPopper, type Instance} from "@popperjs/core";
import {
    DisableDialogModalOkBtn,
    EnableDialogModalOkBtn,
    ShowDialogModal,
    ShowDialogModalHTML,
} from "../components/modalDialog";
import {ShowModalYesNoHTML} from "../components/modalYesNo";
import {AIIsEnabled, AIIsModelEnabled} from "../services/ai";
import {ShowSavedToast, ShowToast} from "../components/toast";
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
            databaseExportSnapshotBtn: document.getElementById("databaseExportSnapshotBtn")! as HTMLButtonElement,
            databaseImportSnapshotBtn: document.getElementById("databaseImportSnapshotBtn")! as HTMLButtonElement,
            databaseSnapshotDirectory: document.getElementById("databaseSnapshotDirectory")! as HTMLDivElement,
            defaultBaseURLBtn: document.getElementById("defaultBaseURLBtn")! as HTMLButtonElement,
            defaultUploadDirectoryBtn: document.getElementById("defaultUploadDirectoryBtn")! as HTMLButtonElement,
            helperVersionText: document.getElementById("helperVersionText")! as HTMLSpanElement,
            indexerServer: document.getElementById("indexerServer")! as HTMLInputElement,
            indexerToken: document.getElementById("indexerToken")! as HTMLInputElement,
            indexerOnBatteryCheckbox: document.getElementById("indexerOnBatteryCheckbox")! as HTMLInputElement,
            indexerRunCheckbox: document.getElementById("indexerRunCheckbox")! as HTMLInputElement,
            indexerStatusText: document.getElementById("indexerStatusText")! as HTMLSpanElement,
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
            saveIpfsPinningBtn: document.getElementById("saveIpfsPinningBtn")! as HTMLButtonElement,
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
                    getIndexerRunning(),
                    getIndexerStatus(),
                    getNetworkPorts(),
                    getIpfsPinning(),
                    getDebugMode(),
                    getServerVersion(),
                    getDatabaseSnapshotDirectory(),
                ]);
            } catch (error) {
                LogError("Error initializing settings page: " + error);
            }

            ExpandAccordionByHash();

            /* Cron Jobs */
            setInterval(getBaseIndexerProgress, 300000); // 5 minutes
            setInterval(getIndexerStatus, 6000); // 1 minute
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
                DOM.baseIndexerProgressUncachedHead.setAttribute("aria-valuenow", latestPercentage.toString());
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
        async function getIndexerRunning() {
            let response = await HttpGetJson("/settings/indexer/running");
            if (response[0] === 200) {
                DOM.indexerRunCheckbox.checked = response[1].indexerRunning;
                DOM.indexerOnBatteryCheckbox.disabled = false;
            }
        }
        async function getIndexerStatus() {
            let response = await HttpGetJson("/settings/indexer/status");
            if (response[0] === 200) {
                let status = DOMPurify.sanitize(response[1].status);
                if (status == "running") {
                    DOM.indexerStatusText.textContent = "Running"
                    DOM.indexerStatusText.style.color = "green";
                } else if (status == "complete") {
                    DOM.indexerStatusText.textContent = "Complete"
                    DOM.indexerStatusText.style.color = "green";
                } else if (status == "failed" || status == "stopped") {
                    DOM.indexerStatusText.textContent = "Stopped / Failed"
                    DOM.indexerStatusText.style.color = "#D3D3D3";
                } else {
                    DOM.indexerStatusText.textContent = status;
                    DOM.indexerStatusText.style.color = "yellow";
                }
            } else {
                LogError("Indexer Status Error");
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
                DOM.ipfsPinningURL.value = response[1].pinningURL;
                DOM.ipfsPinningKey.value = response[1].pinningKey;
            } else {
                ShowDialogModal("Failed to get IPFS Pinning settings");
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
        async function getDatabaseSnapshotDirectory() {
            let response = await HttpGetJson("/settings/database/snapshotDirectory");
            if (response[0] === 200) {
                DOM.databaseSnapshotDirectory.textContent = "Export location: " + response[1].snapshotDirectory;
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
        async function setIndexerRunning() {
            if (DOM.indexerRunCheckbox.checked) {
                let response = await HttpPostJson("/settings/indexer/start", {indexerRun: DOM.indexerRunCheckbox.checked}, DOM.csrfToken.value);
                if (response[0] === 200) {
                    LogInfo(response[1].status);
                    DOM.indexerOnBatteryCheckbox.disabled = false;
                } else {
                    LogError("Indexer Run Error");
                }
            } else {
                let response = await HttpPostJson("/settings/indexer/stop", {indexerRun: DOM.indexerRunCheckbox.checked}, DOM.csrfToken.value);
                if (response[0] === 200) {
                    LogInfo(response[1].status);
                    DOM.indexerOnBatteryCheckbox.disabled = true;
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
        async function setDatabaseExportSnapshot() {
            const originalText = DOM.databaseExportSnapshotBtn.textContent;
            DOM.databaseExportSnapshotBtn.disabled = true;
            DOM.databaseExportSnapshotBtn.innerHTML = '<span class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span> Exporting...';
            const data = {
                snapshot: "export",
            }
            let response = await HttpPostJson("/settings/database/exportSnapshot", data, DOM.csrfToken.value);
            if (response[0] === 200) {
                ShowToast("Database snapshot export started");
            } else {
                ShowDialogModal(response[1].status);
                DOM.databaseExportSnapshotBtn.textContent = originalText;
                DOM.databaseExportSnapshotBtn.disabled = false;
                return
            }
            const interval = setInterval(async() => {
                let response = await HttpGetJson("/settings/database/exportSnapshotStatus");
                if (response[0] === 200) {
                    let status = response[1].status;
                    if (status == "complete") {
                        ShowToast("Database snapshot exported");
                        DOM.databaseExportSnapshotBtn.textContent = originalText;
                        DOM.databaseExportSnapshotBtn.disabled = false;
                        clearInterval(interval);
                    } else if (status == "failed") {
                        ShowToast("Failed to export database snapshot");
                        DOM.databaseExportSnapshotBtn.textContent = originalText;
                        DOM.databaseExportSnapshotBtn.disabled = false;
                        clearInterval(interval);
                    }
                }
            }, 5000)
        }
        async function setDatabaseImportSnapshot() {
            const originalText = DOM.databaseImportSnapshotBtn.textContent;
            DOM.databaseImportSnapshotBtn.disabled = true;
            DOM.databaseImportSnapshotBtn.innerHTML = '<span class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span> Importing...';
            const data = {
                snapshot: "import",
            }
            let response = await HttpPostJson("/settings/database/importSnapshot", data, DOM.csrfToken.value);
            if (response[0] === 200) {
                ShowToast("Database snapshot import started");
            } else {
                ShowDialogModal(response[1].status);
                DOM.databaseImportSnapshotBtn.textContent = originalText;
                DOM.databaseImportSnapshotBtn.disabled = false;
                return
            }
            const interval = setInterval(async() => {
                let response = await HttpGetJson("/settings/database/importSnapshotStatus");
                if (response[0] === 200) {
                    let status = response[1].status;
                    if (status == "complete") {
                        ShowToast("Database snapshot imported");
                        DOM.databaseImportSnapshotBtn.textContent = originalText;
                        DOM.databaseImportSnapshotBtn.disabled = false;
                        clearInterval(interval);
                    } else if (status == "failed") {
                        ShowToast("Failed to import database snapshot");
                        DOM.databaseImportSnapshotBtn.textContent = originalText;
                        DOM.databaseImportSnapshotBtn.disabled = false;
                        clearInterval(interval);
                    }
                }
            }, 5000)
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
                    ShowDialogModal("Uninstalling YourPlace...");
                } else {
                    ShowDialogModal(response[1].message);
                }
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
        DOM.databaseExportSnapshotBtn!.addEventListener("click", setDatabaseExportSnapshot);
        DOM.databaseImportSnapshotBtn!.addEventListener("click", setDatabaseImportSnapshot);
        DOM.defaultBaseURLBtn!.addEventListener("click", setDefaultBaseURL);
        DOM.defaultUploadDirectoryBtn!.addEventListener("click", setDefaultUploadDirectory);
        DOM.saveBaseURLBtn!.addEventListener("click", setBaseURL);
        DOM.saveUploadDirectoryBtn!.addEventListener("click", setUploadDirectory);
        DOM.savePostHistoryDaysBtn!.addEventListener("click", setPostHistoryDays);
        DOM.spiceometerCheck!.addEventListener("change", setSpiceometer);
        DOM.indexerRunCheckbox!.addEventListener("change", setIndexerRunning);
        DOM.indexerOnBatteryCheckbox!.addEventListener("change", setIndexerOnBattery);
        DOM.retestPortsBtn!.addEventListener("click", getNetworkPorts);
        DOM.pinataLI!.addEventListener("click", function(e) {
            DOM.ipfsPinningURL.value = "https://api.pinata.cloud/psa";
            DOM.ipfsPinningKey.value = "";
            ShowDialogModalHTML("Please create an account and an <b>API Key</b> from <a href='https://app.pinata.cloud/' target='_blank'>Pinata here</a><br><br>Then add your <b>JWT (secret access token)</b> to the IPFS Pinning settings page");
        });
        DOM.saveIpfsPinningBtn.addEventListener("click", setIPFSPinning);
        DOM.serverDebugModeCheckbox.addEventListener("change", setDebugMode);
        DOM.serverUpdateBtn.addEventListener("click", getServerUpdates);
        DOM.serverUninstallBtn.addEventListener("click", setUninstall);
        DOM.serverLogsViewBtn.addEventListener("click", getServerLogs);
        DOM.helperLogsViewBtn.addEventListener("click", getHelperLogs);

        init().then();
    }
})();