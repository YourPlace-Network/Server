window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/modalFileUpload.scss";
import {GetAddress, GetChain, WalletPublishFiles} from "../util/blockchain/wallet";
import {FinalizeFiles, UploadFile} from "../util/files";
import {AddFileToIPFS} from "../util/ipfs";
import {ShowDialogModal} from "./modalDialog";
import {IsValidIpfsCid} from "../util/security";
import {ShowToastWithDelay} from "./toast";

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        const DOM = {
            csrfToken: document.getElementById("csrfToken") as HTMLInputElement | null,
            fileInput: document.getElementById("modalFileInput") as HTMLInputElement | null,
            injectedAddress: document.getElementById("injectedAddress") as HTMLInputElement | null,
            injectedBlockchain: document.getElementById("injectedBlockchain") as HTMLInputElement | null,
            list: document.getElementById("modalFileUploadList") as HTMLDivElement | null,
            modalElement: document.getElementById("modalFileUpload") as HTMLDivElement | null,
            openButton: document.getElementById("addFilesButton") as HTMLButtonElement | null,
            submitButton: document.getElementById("modalFileUploadSubmit") as HTMLButtonElement | null,
        };
        if (!DOM.modalElement || !DOM.fileInput || !DOM.submitButton || !DOM.list || !DOM.csrfToken || !DOM.injectedAddress || !DOM.injectedBlockchain) {
            return;
        }
        const csrfToken = DOM.csrfToken;
        const fileInput = DOM.fileInput;
        const injectedAddress = DOM.injectedAddress;
        const injectedBlockchain = DOM.injectedBlockchain;
        const list = DOM.list;
        const modal = new window.bootstrap.Modal(DOM.modalElement, {});
        const submitButton = DOM.submitButton;

        function setSubmitButtonLabel(label: string, iconClass: string) {
            const icon = document.createElement("i");
            icon.className = iconClass;
            const text = document.createElement("span");
            text.textContent = label;
            submitButton.replaceChildren(icon, text);
        }
        function setSubmitButtonSpinner() {
            const spinner = document.createElement("span");
            spinner.classList.add("spinner-border", "spinner-border-sm");
            spinner.setAttribute("aria-hidden", "true");
            const text = document.createElement("span");
            text.classList.add("visually-hidden");
            text.textContent = "Uploading files";
            submitButton.replaceChildren(spinner, text);
        }
        function getResponseStatus(response: any, fallback: string): string {
            return response?.status || fallback;
        }

        function renderSelectedFiles() {
            list.innerHTML = "";
            const files = fileInput.files;
            if (!files || files.length === 0) {
                return;
            }
            for (const file of Array.from(files)) {
                const item = document.createElement("div");
                item.classList.add("modalFileUploadListItem");
                item.textContent = `${file.name} (${file.type || "application/octet-stream"})`;
                list.appendChild(item);
            }
        }

        async function submitFiles() {
            const files = fileInput.files;
            if (!files || files.length === 0) {
                ShowDialogModal("Please choose at least one file");
                return;
            }
            const visibilityInput = document.querySelector("input[name='modalFileVisibility']:checked") as HTMLInputElement | null;
            const visibility = visibilityInput?.value === "private" ? "private" : "public";
            submitButton.disabled = true;
            setSubmitButtonSpinner();
            try {
                const response = await UploadFile(files, csrfToken.value);
                if (response[0] !== 200 || !response[1]?.data || response[1].data.length === 0) {
                    ShowDialogModal(getResponseStatus(response[1], "Failed to upload files"));
                    return;
                }
                const stagedFiles = response[1].data;
                const stagedCids = stagedFiles.map((file: any) => file.cid);
                if (visibility === "private") {
                    const finalizeResponse = await FinalizeFiles(stagedCids, "private", "direct_upload", csrfToken.value);
                    if (finalizeResponse[0] !== 200) {
                        ShowDialogModal(getResponseStatus(finalizeResponse[1], "Failed to save files privately"));
                        return;
                    }
                } else {
                    const activeChain = GetChain();
                    const activeAddress = GetAddress();
                    if (!activeChain || !activeAddress || activeChain !== injectedBlockchain.value || activeAddress !== injectedAddress.value) {
                        ShowDialogModal("Switch your wallet to the current profile before publishing files");
                        return;
                    }
                    const attachments: string[][] = [];
                    for (const stagedFile of stagedFiles) {
                        const cid = await AddFileToIPFS(stagedFile.cid, csrfToken.value);
                        const cidString = cid?.toString() || "";
                        if (!IsValidIpfsCid(cidString)) {
                            ShowDialogModal("Failed to publish one of the files to IPFS");
                            return;
                        }
                        attachments.push([cidString, stagedFile.mimeType, String(stagedFile.size), stagedFile.fileName]);
                    }
                    const txHash = await WalletPublishFiles(attachments);
                    if (!txHash) {
                        ShowDialogModal("Failed to publish files on chain");
                        return;
                    }
                    const finalizeResponse = await FinalizeFiles(stagedCids, "public", "direct_upload", csrfToken.value, txHash, activeChain);
                    if (finalizeResponse[0] !== 200) {
                        ShowDialogModal(getResponseStatus(finalizeResponse[1], "Failed to finalize published files"));
                        return;
                    }
                }
                modal.hide();
                fileInput.value = "";
                renderSelectedFiles();
                window.dispatchEvent(new CustomEvent("filesUploaded"));
                ShowToastWithDelay(visibility === "private" ? "Saved privately on your server." : "Your files should show up shortly. Please wait for them to spread through the network.", 5000);
            } finally {
                submitButton.disabled = false;
                setSubmitButtonLabel("Upload Files", "bi bi-upload modalFileUploadSubmitIcon");
            }
        }

        DOM.openButton?.addEventListener("click", () => {
            fileInput.value = "";
            renderSelectedFiles();
            modal.show();
        });
        fileInput.addEventListener("change", renderSelectedFiles);
        submitButton.addEventListener("click", submitFiles);
    }
})();
