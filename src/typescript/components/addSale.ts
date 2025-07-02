window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/components/addSale.scss";
import {XSSSanitizeValue} from "../util/security";
import {baseCreateMarketplaceListing} from "../util/blockchain/base";
import {Currency, ethToWei} from "../util/currency";
import {HttpPostJson} from "../util/network";
import {UploadFile} from "../util/files";
import {AddFileToIPFS} from "../util/ipfs";
import {GetAddress, IsValidAddress} from "../util/blockchain/wallet";
import {LogError, LogInfo} from "../util/log";

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        let DOM = {
            addSaleModal: document.getElementById("addSaleModal")! as HTMLDivElement,
            addSaleButton: document.getElementById("addSaleButton")! as HTMLButtonElement,
            submitSaleButton: document.getElementById("submitSaleButton")! as HTMLButtonElement,
            saleTitle: document.getElementById("saleTitle")! as HTMLInputElement,
            saleDescription: document.getElementById("saleDescription")! as HTMLTextAreaElement,
            salePrice: document.getElementById("salePrice")! as HTMLInputElement,
            uploadImageButton: document.getElementById("btnUploadImage")! as HTMLButtonElement,
            saleImages: document.getElementById("saleImages")! as HTMLInputElement,
            saleStatusDiv: document.getElementById("saleStatusDiv")! as HTMLDivElement,
            saleStatusText: document.getElementById("saleStatusText")! as HTMLDivElement,
            csrfToken: document.getElementById("csrfToken") as HTMLInputElement,
            saleImagesDiv: document.getElementById("saleImagesDiv")! as HTMLDivElement,
        }

        let tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'));
        tooltipTriggerList.map(function (tooltipTriggerEl) {return new window.bootstrap.Tooltip(tooltipTriggerEl, {delay: {show: 1500, hide: 0}});});
        
        let addSaleModal = new window.bootstrap.Modal(DOM.addSaleModal, {});
        let uploadedImages: fileData[] = [];
        
        interface fileData {
            uuid: string;
            pathOnDisk: string;
            mimeType: string;
            fileName: string;
            size: string;
            fileUrl?: string;
        }

        // Initialize by hiding the add sale button (will be shown when marketplace view is active)
        DOM.addSaleButton.style.display = "none";

        // Event Handlers
        DOM.addSaleButton.addEventListener("click", function() {
            resetForm();
            addSaleModal.show();
        });
        DOM.submitSaleButton.addEventListener("click", async function() {
            await submitListing();
        });
        DOM.uploadImageButton.addEventListener("click", function() {
            DOM.saleImages.click();
        });
        DOM.saleImages.addEventListener("change", function() {
            let files = DOM.saleImages.files;
            if (files && files.length > 0) {
                handleImageUpload(files);
            }
        });
        DOM.salePrice.addEventListener("input", function() {
            updatePriceDisplay();
        });

        // Functions
        function resetForm() {
            DOM.saleTitle.value = "";
            DOM.saleDescription.value = "";
            DOM.salePrice.value = "";
            DOM.saleImagesDiv.innerHTML = "";
            DOM.saleStatusDiv.style.display = "none";
            uploadedImages = [];
        }
        function updatePriceDisplay() {
            let price = parseFloat(DOM.salePrice.value);
            if (isNaN(price) || price <= 0) {
                DOM.saleStatusText.textContent = "";
                DOM.saleStatusDiv.style.display = "none";
                return;
            }
            
            let currency = new Currency("ETH", price.toString(), "", "base");
            DOM.saleStatusText.textContent = currency.formatDisplay();
            DOM.saleStatusDiv.style.display = "block";
        }
        async function handleImageUpload(files: FileList) {
            for (let i = 0; i < files.length; i++) {
                let file = files[i];
                if (!file.type.startsWith('image/')) {
                    LogError("Only image files are allowed");
                    continue;
                }
                try {
                    let [statusCode, responseData] = await UploadFile(file, DOM.csrfToken.value);
                    if (statusCode === 200 && responseData.status === "success") {
                        let fileData: fileData = {
                            uuid: responseData.uuid,
                            pathOnDisk: responseData.pathOnDisk,
                            mimeType: file.type,
                            fileName: file.name,
                            size: file.size.toString(),
                            fileUrl: `/file/${responseData.uuid}`
                        };
                        uploadedImages.push(fileData);
                        createImagePreview(fileData);
                    } else {
                        LogError("Upload failed: " + (responseData.error || "Unknown error"));
                    }
                } catch (error) {
                    LogError("Failed to upload image: " + error);
                }
            }
        }
        function createImagePreview(fileData: fileData) {
            let previewDiv = document.createElement("div");
            previewDiv.className = "image-preview";
            previewDiv.innerHTML = `
                <img src="${fileData.fileUrl}" alt="${XSSSanitizeValue(fileData.fileName)}" class="preview-image">
                <div class="preview-controls">
                    <button type="button" class="btn btn-sm btn-danger remove-image">Remove</button>
                </div>
            `;
            
            let removeBtn = previewDiv.querySelector('.remove-image') as HTMLButtonElement;
            removeBtn.addEventListener('click', () => {
                uploadedImages = uploadedImages.filter(img => img.uuid !== fileData.uuid);
                previewDiv.remove();
            });
            
            DOM.saleImagesDiv.appendChild(previewDiv);
        }
        async function submitListing() {
            let title = XSSSanitizeValue(DOM.saleTitle.value.trim());
            let description = XSSSanitizeValue(DOM.saleDescription.value.trim());
            let price = parseFloat(DOM.salePrice.value);
            if (!title || title.length === 0) {
                LogError("Product title is required");
                return;
            }
            if (title.length > 200) {
                LogError("Title is too long (max 200 characters)");
                return;
            }
            if (!description || description.length === 0) {
                LogError("Product description is required");
                return;
            }
            if (description.length > 1000) {
                LogError("Description is too long (max 1000 characters)");
                return;
            }
            if (isNaN(price) || price <= 0) {
                LogError("Valid price is required");
                return;
            }
            let address = GetAddress();
            if (!address || !IsValidAddress(address)) {
                LogError("Please connect your wallet first");
                return;
            }
            try {
                DOM.submitSaleButton.disabled = true;
                DOM.submitSaleButton.textContent = "Creating Listing...";
                let currency = new Currency("ETH", price.toString(), "", "base");
                let priceSmallUnit = currency.convertToSmallUnit();
                let currencySymbol = "ETH";
                let imageUrls: string[] = [];
                for (let fileData of uploadedImages) {
                    if (fileData.fileUrl) {
                        imageUrls.push(fileData.fileUrl);
                    }
                }
                let txHash = await baseCreateMarketplaceListing(title, description, price.toString(), priceSmallUnit, currencySymbol, imageUrls);
                if (txHash) {
                    LogInfo("Marketplace listing created with transaction: " + txHash);
                    addSaleModal.hide();
                    resetForm();
                    alert("Listing created successfully! Transaction: " + txHash);
                } else {
                    throw new Error("Failed to create blockchain transaction");
                }
            } catch (error) {
                LogError("Failed to create marketplace listing: " + error);
                alert("Failed to create listing. Please try again.");
            } finally {
                DOM.submitSaleButton.disabled = false;
                DOM.submitSaleButton.textContent = "List Product";
            }
        }
    }
})();