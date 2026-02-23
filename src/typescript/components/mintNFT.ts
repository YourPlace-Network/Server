window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/mintNFT.scss";
import {GetWallet, WalletMintCollectible} from "../util/blockchain/wallet";
import {UploadFile} from "../util/files";
import {AddFileToIPFS, UploadToIPFSService} from "../util/ipfs";
import {ShowDialogModalHTML} from "./modalDialog";
import {ShowToast} from "./toast";
import {LogError} from "../util/log";

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}
    function main() {
        let DOM = {
            csrfToken: document.getElementById("csrfToken") as HTMLInputElement,
            gatewayMintEnabled: document.getElementById("gatewayMintEnabled") as HTMLInputElement,
            gatewayMode: document.getElementById("gatewayModeMintNFT") as HTMLInputElement,
            mintNFTButton: document.getElementById("mintNFTButton")! as HTMLButtonElement,
            mintNFTDescription: document.getElementById("mintNFTDescription")! as HTMLTextAreaElement,
            mintNFTFileInput: document.getElementById("mintNFTFileInput")! as HTMLInputElement,
            mintNFTModal: document.getElementById("modalMintNFT")! as HTMLDivElement,
            mintNFTName: document.getElementById("mintNFTName")! as HTMLInputElement,
            mintNFTPreviewDiv: document.getElementById("mintNFTPreviewDiv")! as HTMLDivElement,
            mintNFTRoyalty: document.getElementById("mintNFTRoyalty")! as HTMLInputElement,
            mintNFTSubmitBtn: document.getElementById("mintNFTSubmitBtn")! as HTMLButtonElement,
        }
        let mintModal = new window.bootstrap.Modal(DOM.mintNFTModal, {});
        let uploadedFileCid = "";
        let uploadedFileUUID = "";
        let uploadedFileMimeType = "";
        function isGatewayMintEnabled(): boolean {
            return DOM.gatewayMintEnabled?.value === "true";
        }
        function isLocalhost(): boolean {
            const hostname = window.location.hostname;
            return hostname === "localhost" || hostname === "127.0.0.1" || hostname === "[::1]";
        }
        function isGatewayMode(): boolean {
            return DOM.gatewayMode && DOM.gatewayMode.value === "true" && !isLocalhost();
        }
        function resetForm() {
            DOM.mintNFTName.value = "";
            DOM.mintNFTDescription.value = "";
            DOM.mintNFTRoyalty.value = "5";
            DOM.mintNFTFileInput.value = "";
            DOM.mintNFTPreviewDiv.innerHTML = "";
            uploadedFileCid = "";
            uploadedFileUUID = "";
            uploadedFileMimeType = "";
        }
        function showModal() {
            if (!GetWallet()) {
                window.location.href = "/login";
                return;
            }
            if (isGatewayMode() && !isGatewayMintEnabled()) {
                ShowDialogModalHTML(
                    "To create a collectible, you need to host your own YourPlace server.<br><br>" +
                    '<a href="https://yourplace.network/download" target="_blank" rel="noopener noreferrer">Download YourPlace</a>'
                );
                return;
            }
            resetForm();
            mintModal.show();
        }
        DOM.mintNFTFileInput.addEventListener("change", async () => {
            const files = DOM.mintNFTFileInput.files;
            if (!files || files.length === 0) return;
            if (isGatewayMode() && !isGatewayMintEnabled()) {
                DOM.mintNFTFileInput.value = "";
                ShowDialogModalHTML(
                    "To create a collectible, you need to host your own YourPlace server.<br><br>" +
                    '<a href="https://yourplace.network/download" target="_blank" rel="noopener noreferrer">Download YourPlace</a>'
                );
                return;
            }
            const file = files[0];
            const csrfToken = DOM.csrfToken ? DOM.csrfToken.value : "";
            if (isGatewayMintEnabled()) {
                const cid = await UploadToIPFSService(file, csrfToken);
                if (!cid) {
                    ShowToast("Failed to upload file");
                    return;
                }
                uploadedFileCid = cid;
            } else {
                const response = await UploadFile(file, csrfToken);
                if (response[0] !== 200 || !response[1].data || !response[1].data[0] || !response[1].data[0].uuid) {
                    ShowToast("Failed to upload file");
                    return;
                }
                uploadedFileUUID = response[1].data[0].uuid;
            }
            uploadedFileMimeType = file.type;
            DOM.mintNFTPreviewDiv.innerHTML = "";
            if (file.type.startsWith("video/")) {
                let video = document.createElement("video");
                video.src = URL.createObjectURL(file);
                video.controls = true;
                video.muted = true;
                DOM.mintNFTPreviewDiv.appendChild(video);
            } else {
                let img = document.createElement("img");
                img.src = URL.createObjectURL(file);
                img.alt = "Preview";
                DOM.mintNFTPreviewDiv.appendChild(img);
            }
        });
        DOM.mintNFTSubmitBtn.addEventListener("click", async () => {
            const name = DOM.mintNFTName.value.trim();
            if (!name) {
                ShowToast("Please enter a name");
                return;
            }
            if (!uploadedFileUUID && !uploadedFileCid) {
                ShowToast("Please upload media");
                return;
            }
            const csrfToken = DOM.csrfToken ? DOM.csrfToken.value : "";
            DOM.mintNFTSubmitBtn.disabled = true;
            DOM.mintNFTSubmitBtn.textContent = "Creating...";
            try {
                let mediaCidStr: string;
                if (isGatewayMintEnabled()) {
                    mediaCidStr = uploadedFileCid;
                } else {
                    const mediaCid = await AddFileToIPFS(uploadedFileUUID, csrfToken);
                    if (!mediaCid) {
                        ShowToast("Failed to pin media to IPFS");
                        return;
                    }
                    mediaCidStr = mediaCid.toString();
                }
                const description = DOM.mintNFTDescription.value.trim();
                const royalty = parseInt(DOM.mintNFTRoyalty.value) || 5;
                const metadata = {
                    name: name,
                    description: description,
                    image: "ipfs://" + mediaCidStr,
                    image_mimetype: uploadedFileMimeType,
                    royalty_percentage: royalty,
                };
                const metadataBlob = new Blob([JSON.stringify(metadata)], {type: "application/json"});
                const metadataFile = new File([metadataBlob], "metadata.json", {type: "application/json"});
                let metadataCidStr: string;
                if (isGatewayMintEnabled()) {
                    const metaCid = await UploadToIPFSService(metadataFile, csrfToken);
                    if (!metaCid) {
                        ShowToast("Failed to upload metadata");
                        return;
                    }
                    metadataCidStr = metaCid;
                } else {
                    const metaResponse = await UploadFile(metadataFile, csrfToken);
                    if (metaResponse[0] !== 200 || !metaResponse[1].data || !metaResponse[1].data[0] || !metaResponse[1].data[0].uuid) {
                        ShowToast("Failed to upload metadata");
                        return;
                    }
                    const metadataCid = await AddFileToIPFS(metaResponse[1].data[0].uuid, csrfToken);
                    if (!metadataCid) {
                        ShowToast("Failed to pin metadata to IPFS");
                        return;
                    }
                    metadataCidStr = metadataCid.toString();
                }
                const metadataUri = "ipfs://" + metadataCidStr;
                const unitName = name.substring(0, 8).toUpperCase().replace(/[^A-Z0-9]/g, "");
                const success = await WalletMintCollectible(metadataUri, name, unitName);
                if (success) {
                    ShowToast("Collectible created!");
                    mintModal.hide();
                    resetForm();
                    if ((window as any).CollectibleMintCallback) {
                        (window as any).CollectibleMintCallback();
                    }
                } else {
                    ShowToast("Failed to mint collectible");
                }
            } catch (error) {
                LogError("mintNFT submit error: " + error);
                ShowToast("Error creating collectible");
            } finally {
                DOM.mintNFTSubmitBtn.disabled = false;
                DOM.mintNFTSubmitBtn.textContent = "Create";
            }
        });
        DOM.mintNFTButton.addEventListener("click", showModal);
    }
})();
