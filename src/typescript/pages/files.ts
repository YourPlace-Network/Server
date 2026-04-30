window.bootstrap = require("bootstrap/dist/js/bootstrap.bundle");
import "../../scss/global.scss";
import "../../scss/pages/files.scss";
import "../components/menu";
import "../components/modalDialog";
import "../components/modalFileUpload";
import "../components/modalYesNo";
import {ShowDialogModal} from "../components/modalDialog";
import {ShowModalYesNo} from "../components/modalYesNo";
import {GetAddress, GetChain, WalletDeleteFiles, WalletGetAvatar, WalletGetCachedAvatar, WalletPublishFiles} from "../util/blockchain/wallet";
import {HttpGetJson} from "../util/network";
import {CIDToSubdomainURL} from "../util/ipfs";
import {DeleteFile, formatFileSize, PrepareRenameFile, RenameFile} from "../util/files";
import {ShowToastWithDelay} from "../components/toast";

type FileRow = {
    addedDate: number;
    canDelete?: boolean;
    cid: string;
    fileName: string;
    mimeType: string;
    ownerAddress: string;
    ownerBlockchain: string;
    size: number;
    source: string;
    txHash?: string;
    visibility: "public" | "private";
    loadOrder?: number;
};
type SortDirection = "asc" | "desc" | null;
type SortKey = "addedDate" | "cid" | "fileName" | "mimeType" | "size" | "source" | "visibility";

(function initialize() {
    if (document.readyState === "loading") {document.addEventListener("DOMContentLoaded", main);} else {main();}

    function main() {
        const DOM = {
            filesAvatar: document.getElementById("filesAvatar") as HTMLImageElement,
            filesEmptyState: document.getElementById("filesEmptyState") as HTMLDivElement,
            modalFileRename: document.getElementById("modalFileRename") as HTMLDivElement,
            modalFileRenameBaseName: document.getElementById("modalFileRenameBaseName") as HTMLInputElement,
            modalFileRenameCurrentName: document.getElementById("modalFileRenameCurrentName") as HTMLSpanElement,
            modalFileRenameExtension: document.getElementById("modalFileRenameExtension") as HTMLSpanElement,
            modalFileRenameSubmit: document.getElementById("modalFileRenameSubmit") as HTMLButtonElement,
            filesSearch: document.getElementById("filesSearch") as HTMLInputElement,
            filesTable: document.getElementById("filesTable") as HTMLTableElement,
            filesTableBody: document.getElementById("filesTableBody") as HTMLTableSectionElement,
            filesTableWrap: document.querySelector(".filesTableWrap") as HTMLDivElement,
            injectedAddress: document.getElementById("injectedAddress") as HTMLInputElement,
            injectedBlockchain: document.getElementById("injectedBlockchain") as HTMLInputElement,
            isGuest: document.getElementById("isGuest") as HTMLInputElement,
            preview: document.createElement("div"),
            csrfToken: document.getElementById("csrfToken") as HTMLInputElement,
            sortableHeaders: Array.from(document.querySelectorAll(".filesSortableHeader")) as HTMLTableCellElement[],
        };
        let allFiles: FileRow[] = [];
        let activeActionButton: HTMLButtonElement | null = null;
        let activeActionMenu: HTMLDivElement | null = null;
        let activeActionMenuCleanup: (() => void) | null = null;
        let previewTimer: number | null = null;
        let renameTarget: FileRow | null = null;
        let sortKey: SortKey | null = null;
        let sortDirection: SortDirection = null;
        const renameModal = new window.bootstrap.Modal(DOM.modalFileRename, {});

        function getSourceLabel(source: string): string {
            if (!source) {
                return "";
            }
            return source.split("_").map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(" ");
        }
        function getFileNameParts(fileName: string): {baseName: string; extension: string} {
            const lastDot = fileName.lastIndexOf(".");
            if (lastDot <= 0) {
                return {baseName: fileName, extension: ""};
            }
            return {
                baseName: fileName.substring(0, lastDot),
                extension: fileName.substring(lastDot),
            };
        }
        function setRenameButtonLabel(label: string, iconClass: string) {
            const icon = document.createElement("i");
            icon.className = iconClass;
            const text = document.createElement("span");
            text.textContent = label;
            DOM.modalFileRenameSubmit.replaceChildren(icon, text);
        }
        function buildActionLabel(iconClass: string, label: string): DocumentFragment {
            const fragment = document.createDocumentFragment();
            const icon = document.createElement("i");
            icon.className = `${iconClass} filesActionDropdownIcon`;
            icon.setAttribute("aria-hidden", "true");
            const text = document.createElement("span");
            text.className = "filesActionDropdownLabel";
            text.textContent = label;
            fragment.append(icon, text);
            return fragment;
        }
        function positionActionMenu(actionButton: HTMLButtonElement, actionMenu: HTMLDivElement) {
            const buttonRect = actionButton.getBoundingClientRect();
            const menuRect = actionMenu.getBoundingClientRect();
            const viewportPadding = 8;
            const preferredLeft = buttonRect.right - menuRect.width;
            const left = Math.max(viewportPadding, Math.min(preferredLeft, window.innerWidth - menuRect.width - viewportPadding));
            const spaceBelow = window.innerHeight - buttonRect.bottom - viewportPadding;
            const spaceAbove = buttonRect.top - viewportPadding;
            const openUpward = menuRect.height > spaceBelow && spaceAbove > spaceBelow;
            const top = openUpward
                ? Math.max(viewportPadding, buttonRect.top - menuRect.height - 6)
                : Math.min(window.innerHeight - menuRect.height - viewportPadding, buttonRect.bottom + 6);
            actionMenu.style.left = `${left}px`;
            actionMenu.style.top = `${top}px`;
        }
        function closeActionMenu() {
            if (activeActionMenuCleanup) {
                activeActionMenuCleanup();
                activeActionMenuCleanup = null;
            }
            if (activeActionMenu) {
                activeActionMenu.remove();
                activeActionMenu = null;
            }
            if (activeActionButton) {
                activeActionButton.setAttribute("aria-expanded", "false");
                activeActionButton = null;
            }
        }
        function createActionMenuButton(iconClass: string, label: string, onClick: () => void | Promise<void>, disabled?: boolean, title?: string): HTMLButtonElement {
            const button = document.createElement("button");
            button.classList.add("filesActionDropdownItem");
            button.type = "button";
            button.disabled = disabled === true;
            button.title = title || label;
            button.appendChild(buildActionLabel(iconClass, label));
            button.addEventListener("click", async () => {
                if (button.disabled) {
                    return;
                }
                closeActionMenu();
                await onClick();
            });
            return button;
        }
        function openActionMenu(row: FileRow, actionButton: HTMLButtonElement, renameDisabledReason: string) {
            if (activeActionButton === actionButton) {
                closeActionMenu();
                return;
            }
            closeActionMenu();
            const actionMenu = document.createElement("div");
            actionMenu.classList.add("filesActionDropdown", "filesActionDropdownFloating");
            actionMenu.appendChild(createActionMenuButton("bi bi-copy", "Copy", async () => {
                await copyCidLink(row);
            }));
            actionMenu.appendChild(createActionMenuButton("bi bi-pencil-square", "Rename", () => {
                openRenameModal(row);
            }, renameDisabledReason !== "", renameDisabledReason || "Rename file"));
            actionMenu.appendChild(createActionMenuButton("bi bi-trash", "Delete", async () => {
                const confirmed = await ShowModalYesNo(`Delete ${row.fileName}?`, {centerContent: true});
                if (!confirmed) {
                    return;
                }
                await handleDelete(row);
            }, row.canDelete === false, row.canDelete === false ? "File already removed locally" : "Delete file"));
            document.body.appendChild(actionMenu);
            positionActionMenu(actionButton, actionMenu);
            actionButton.setAttribute("aria-expanded", "true");
            const handlePointerDown = (event: Event) => {
                const target = event.target as Node | null;
                if (!target) {
                    return;
                }
                if (actionMenu.contains(target) || actionButton.contains(target)) {
                    return;
                }
                closeActionMenu();
            };
            const handleEscape = (event: KeyboardEvent) => {
                if (event.key === "Escape") {
                    closeActionMenu();
                }
            };
            const handleViewportChange = () => {
                closeActionMenu();
            };
            document.addEventListener("mousedown", handlePointerDown);
            window.addEventListener("resize", handleViewportChange);
            window.addEventListener("scroll", handleViewportChange, true);
            activeActionButton = actionButton;
            activeActionMenu = actionMenu;
            activeActionMenuCleanup = () => {
                document.removeEventListener("mousedown", handlePointerDown);
                document.removeEventListener("keydown", handleEscape);
                window.removeEventListener("resize", handleViewportChange);
                window.removeEventListener("scroll", handleViewportChange, true);
            };
            document.addEventListener("keydown", handleEscape);
        }
        function resetRenameModal() {
            renameTarget = null;
            DOM.modalFileRenameBaseName.value = "";
            DOM.modalFileRenameCurrentName.textContent = "";
            DOM.modalFileRenameExtension.textContent = "";
            DOM.modalFileRenameExtension.style.display = "none";
            DOM.modalFileRenameSubmit.disabled = false;
            setRenameButtonLabel("Rename File", "bi bi-pencil-square filesRenameSubmitIcon");
        }
        function openRenameModal(row: FileRow) {
            const {baseName, extension} = getFileNameParts(row.fileName);
            renameTarget = row;
            DOM.modalFileRenameBaseName.value = baseName;
            DOM.modalFileRenameCurrentName.textContent = row.fileName;
            DOM.modalFileRenameExtension.textContent = extension;
            DOM.modalFileRenameExtension.style.display = extension.length > 0 ? "" : "none";
            renameModal.show();
            window.setTimeout(() => {
                DOM.modalFileRenameBaseName.focus();
                DOM.modalFileRenameBaseName.select();
            }, 50);
        }
        async function renderFilesAvatar() {
            const defaultAvatar = "/static/image/avatar.png";
            DOM.filesAvatar.onerror = () => {
                DOM.filesAvatar.src = defaultAvatar;
                DOM.filesAvatar.onerror = null;
            };
            const cachedAvatar = WalletGetCachedAvatar(DOM.injectedBlockchain.value, DOM.injectedAddress.value);
            DOM.filesAvatar.src = cachedAvatar || defaultAvatar;
            const avatarUrl = await WalletGetAvatar(DOM.injectedBlockchain.value, DOM.injectedAddress.value);
            DOM.filesAvatar.src = avatarUrl || defaultAvatar;
        }
        function hidePreview() {
            if (previewTimer !== null) {
                window.clearTimeout(previewTimer);
                previewTimer = null;
            }
            DOM.preview.classList.remove("visible");
            DOM.preview.innerHTML = "";
        }
        function getOpenUrl(row: FileRow): string {
            return row.visibility === "private" ? `/files/preview/${encodeURIComponent(row.cid)}` : CIDToSubdomainURL(row.cid);
        }
        function getPreviewUrl(row: FileRow): string {
            return row.visibility === "private" ? `/files/preview/${encodeURIComponent(row.cid)}` : CIDToSubdomainURL(row.cid);
        }
        function renderPreviewContent(row: FileRow): HTMLElement {
            const previewUrl = getPreviewUrl(row);
            if (row.mimeType.startsWith("image/")) {
                const image = document.createElement("img");
                image.classList.add("filesPreviewMedia");
                image.src = previewUrl;
                image.alt = row.fileName;
                return image;
            }
            if (row.mimeType.startsWith("video/")) {
                const video = document.createElement("video");
                video.classList.add("filesPreviewMedia");
                video.src = previewUrl;
                video.muted = true;
                video.autoplay = true;
                video.loop = true;
                video.playsInline = true;
                return video;
            }
            const meta = document.createElement("div");
            meta.classList.add("filesPreviewMeta");
            const name = document.createElement("div");
            name.classList.add("filesPreviewName");
            name.textContent = row.fileName;
            const type = document.createElement("div");
            type.classList.add("filesPreviewType");
            type.textContent = row.mimeType || "Unknown file type";
            const fallback = document.createElement("div");
            fallback.classList.add("filesPreviewFallback");
            fallback.textContent = "Preview unavailable";
            meta.append(name, type, fallback);
            return meta;
        }
        function showPreview(row: FileRow, anchor: HTMLElement) {
            const previewUrl = getPreviewUrl(row);
            if (!previewUrl) {
                return;
            }
            const rect = anchor.getBoundingClientRect();
            DOM.preview.replaceChildren(renderPreviewContent(row));
            DOM.preview.classList.add("visible");
            DOM.preview.style.left = `${Math.max(8, Math.min(rect.left, window.innerWidth - 280))}px`;
            DOM.preview.style.top = `${Math.max(8, Math.min(rect.bottom + 12, window.innerHeight - 220))}px`;
        }
        function queuePreview(row: FileRow, anchor: HTMLElement) {
            hidePreview();
            previewTimer = window.setTimeout(() => {
                showPreview(row, anchor);
                previewTimer = null;
            }, 1000);
        }
        async function handleDelete(row: FileRow) {
            let txHash = "";
            let blockchain = "";
            if (row.visibility === "public") {
                const activeChain = GetChain();
                const activeAddress = GetAddress();
                if (!activeChain || !activeAddress || activeChain !== DOM.injectedBlockchain.value || activeAddress !== DOM.injectedAddress.value) {
                    ShowDialogModal("Switch your wallet to the current profile before deleting public files");
                    return;
                }
                const submittedTxHash = await WalletDeleteFiles([row.cid]);
                if (!submittedTxHash) {
                    ShowDialogModal("Failed to publish file deletion on chain");
                    return;
                }
                txHash = submittedTxHash;
                blockchain = activeChain;
            }
            const [status, response] = await DeleteFile(row.cid, DOM.csrfToken.value, txHash, blockchain);
            if (status !== 200) {
                ShowDialogModal(response?.status || "Failed to delete file");
                return;
            }
            hidePreview();
            allFiles = allFiles.filter((candidate) => !(candidate.cid === row.cid && candidate.visibility === row.visibility));
            await renderFiles();
        }
        async function copyCidLink(row: FileRow) {
            const cidLink = `ipfs://${row.cid}`;
            try {
                if (navigator.clipboard && window.isSecureContext) {
                    await navigator.clipboard.writeText(cidLink);
                } else {
                    const tempInput = document.createElement("textarea");
                    tempInput.value = cidLink;
                    tempInput.setAttribute("readonly", "true");
                    tempInput.style.position = "absolute";
                    tempInput.style.left = "-9999px";
                    document.body.appendChild(tempInput);
                    tempInput.select();
                    document.execCommand("copy");
                    document.body.removeChild(tempInput);
                }
                ShowToastWithDelay("Copied CID link", 1000);
            } catch {
                ShowDialogModal("Failed to copy CID link");
            }
        }
        async function handleRename() {
            if (!renameTarget) {
                return;
            }
            const row = renameTarget;
            const fileNameBase = DOM.modalFileRenameBaseName.value.trim();
            if (fileNameBase.length === 0) {
                ShowDialogModal("Please enter a new file name");
                return;
            }
            DOM.modalFileRenameSubmit.disabled = true;
            setRenameButtonLabel("Renaming...", "bi bi-pencil-square filesRenameSubmitIcon");
            try {
                const [prepareStatus, prepareResponse] = await PrepareRenameFile(row.cid, fileNameBase, DOM.csrfToken.value);
                if (prepareStatus !== 200 || !prepareResponse?.fileName || !prepareResponse?.cid) {
                    ShowDialogModal(prepareResponse?.status || "Failed to prepare file rename");
                    return;
                }
                if (row.visibility === "public") {
                    const activeChain = GetChain();
                    const activeAddress = GetAddress();
                    if (!activeChain || !activeAddress || activeChain !== DOM.injectedBlockchain.value || activeAddress !== DOM.injectedAddress.value) {
                        ShowDialogModal("Switch your wallet to the current profile before renaming public files");
                        return;
                    }
                    const deleteTxHash = await WalletDeleteFiles([row.cid]);
                    if (!deleteTxHash) {
                        ShowDialogModal("Failed to publish file rename delete transaction on chain");
                        return;
                    }
                    const publishTxHash = await WalletPublishFiles([[prepareResponse.cid, row.mimeType, String(prepareResponse.size || row.size), prepareResponse.fileName]]);
                    if (!publishTxHash) {
                        ShowDialogModal("File delete was submitted on chain, but publishing the renamed file failed");
                        return;
                    }
                    const [renameStatus, renameResponse] = await RenameFile(row.cid, fileNameBase, DOM.csrfToken.value, prepareResponse.cid, deleteTxHash, publishTxHash, activeChain);
                    if (renameStatus !== 200) {
                        ShowDialogModal(renameResponse?.status || "Failed to finalize public file rename");
                        return;
                    }
                } else {
                    const [renameStatus, renameResponse] = await RenameFile(row.cid, fileNameBase, DOM.csrfToken.value);
                    if (renameStatus !== 200) {
                        ShowDialogModal(renameResponse?.status || "Failed to rename file");
                        return;
                    }
                }
                renameModal.hide();
                resetRenameModal();
                await loadFiles();
            } finally {
                DOM.modalFileRenameSubmit.disabled = false;
                setRenameButtonLabel("Rename File", "bi bi-pencil-square filesRenameSubmitIcon");
            }
        }

        function isGuest(): boolean {
            return DOM.isGuest?.value === "true";
        }
        function compareText(left: string, right: string, direction: Exclude<SortDirection, null>): number {
            const result = left.localeCompare(right);
            return direction === "asc" ? result : -result;
        }
        function compareNumber(left: number, right: number, direction: Exclude<SortDirection, null>): number {
            if (left === right) {
                return 0;
            }
            if (direction === "asc") {
                return left < right ? -1 : 1;
            }
            return left > right ? -1 : 1;
        }
        function getSortArrow(headerSortKey: SortKey): string {
            if (sortKey !== headerSortKey || sortDirection === null) {
                return "";
            }
            return sortDirection === "asc" ? "↑" : "↓";
        }
        function updateSortHeaders() {
            DOM.sortableHeaders.forEach((header) => {
                const headerKey = header.dataset.sortKey as SortKey | undefined;
                const arrow = header.querySelector(".filesSortArrow") as HTMLSpanElement | null;
                if (!headerKey || !arrow) {
                    return;
                }
                const isActive = sortKey === headerKey && sortDirection !== null;
                header.classList.toggle("active", isActive);
                arrow.textContent = getSortArrow(headerKey);
            });
        }
        function cycleSort(nextKey: SortKey) {
            if (sortKey !== nextKey) {
                sortKey = nextKey;
                sortDirection = "desc";
            } else if (sortDirection === "desc") {
                sortDirection = "asc";
            } else if (sortDirection === "asc") {
                sortKey = null;
                sortDirection = null;
            } else {
                sortDirection = "desc";
            }
            updateSortHeaders();
            renderFiles().then();
        }

        function getFilteredFiles(): FileRow[] {
            const search = DOM.filesSearch.value.trim().toLowerCase();
            let rows = [...allFiles];
            if (search.length > 0) {
                rows = rows.filter((row) => row.fileName.toLowerCase().includes(search) || row.cid.toLowerCase().includes(search));
            }
            if (sortKey === null || sortDirection === null) {
                rows.sort((left, right) => (left.loadOrder || 0) - (right.loadOrder || 0));
                return rows;
            }
            const activeSortDirection: Exclude<SortDirection, null> = sortDirection;
            rows.sort((left, right) => {
                let result = 0;
                switch (sortKey) {
                    case "fileName":
                        result = compareText(left.fileName, right.fileName, activeSortDirection);
                        break;
                    case "cid":
                        result = compareText(left.cid, right.cid, activeSortDirection);
                        break;
                    case "mimeType":
                        result = compareText(left.mimeType, right.mimeType, activeSortDirection);
                        break;
                    case "size":
                        result = compareNumber(left.size, right.size, activeSortDirection);
                        break;
                    case "visibility":
                        result = compareText(left.visibility, right.visibility, activeSortDirection);
                        break;
                    case "source":
                        result = compareText(getSourceLabel(left.source), getSourceLabel(right.source), activeSortDirection);
                        break;
                    case "addedDate":
                    default:
                        result = compareNumber(left.addedDate, right.addedDate, activeSortDirection);
                        break;
                }
                if (result !== 0) {
                    return result;
                }
                return (left.loadOrder || 0) - (right.loadOrder || 0);
            });
            return rows;
        }

        async function renderFiles() {
            const rows = getFilteredFiles();
            closeActionMenu();
            hidePreview();
            DOM.filesTableBody.innerHTML = "";
            DOM.filesEmptyState.style.display = rows.length === 0 ? "block" : "none";
            DOM.filesTable.style.display = rows.length === 0 ? "none" : "table";
            for (const row of rows) {
                const tr = document.createElement("tr");
                const fileSize = await formatFileSize(row.size || 0);
                const values: Array<{className: string; value: string}> = [
                    {className: "filesTypeCell", value: row.mimeType},
                    {className: "filesSizeCell", value: fileSize},
                    {className: "filesUploadedCell", value: new Date(row.addedDate * 1000).toLocaleString()},
                    {className: "filesVisibilityCell", value: row.visibility},
                    {className: "filesSourceCell", value: getSourceLabel(row.source)},
                ];
                const nameCell = document.createElement("td");
                nameCell.classList.add("filesNameCell");
                nameCell.textContent = row.fileName;
                tr.appendChild(nameCell);
                const cidCell = document.createElement("td");
                cidCell.classList.add("filesCidCell");
                const cidLink = document.createElement("a");
                cidLink.classList.add("filesCidLink");
                cidLink.href = getOpenUrl(row);
                cidLink.rel = "noopener noreferrer";
                cidLink.target = "_blank";
                cidLink.textContent = row.cid;
                cidLink.addEventListener("mouseenter", () => queuePreview(row, cidLink));
                cidLink.addEventListener("mouseleave", hidePreview);
                cidCell.appendChild(cidLink);
                tr.appendChild(cidCell);
                values.forEach(({className, value}) => {
                    const td = document.createElement("td");
                    td.classList.add(className);
                    td.textContent = value;
                    tr.appendChild(td);
                });
                if (!isGuest()) {
                    const actionCell = document.createElement("td");
                    actionCell.classList.add("filesActionCell");
                    let renameDisabledReason = "";
                    if (row.canDelete === false) {
                        renameDisabledReason = "Local file copy required to rename";
                    } else if (row.visibility === "public" && row.source !== "direct_upload") {
                        renameDisabledReason = "Published attachments cannot be renamed";
                    }
                    const actionMenu = document.createElement("div");
                    actionMenu.classList.add("dropdown");
                    const actionButton = document.createElement("button");
                    actionButton.classList.add("btn", "btn-sm", "filesActionMenuButton");
                    actionButton.type = "button";
                    actionButton.setAttribute("aria-expanded", "false");
                    actionButton.setAttribute("aria-label", "File actions");
                    actionButton.title = "File actions";
                    const actionIcon = document.createElement("i");
                    actionIcon.classList.add("bi", "bi-list");
                    actionButton.appendChild(actionIcon);
                    actionButton.addEventListener("click", (event) => {
                        event.preventDefault();
                        event.stopPropagation();
                        openActionMenu(row, actionButton, renameDisabledReason);
                    });
                    actionMenu.append(actionButton);
                    actionCell.appendChild(actionMenu);
                    tr.appendChild(actionCell);
                }
                DOM.filesTableBody.appendChild(tr);
            }
        }

        async function loadFiles() {
            const [status, response] = await HttpGetJson(`/files/data/${encodeURIComponent(DOM.injectedBlockchain.value)}/${encodeURIComponent(DOM.injectedAddress.value)}`);
            if (status !== 200 || !response?.files) {
                DOM.filesEmptyState.style.display = "block";
                return;
            }
            allFiles = (response.files as FileRow[]).map((file, index) => ({...file, loadOrder: index}));
            updateSortHeaders();
            await renderFiles();
        }

        DOM.filesSearch.addEventListener("input", () => { renderFiles().then(); });
        DOM.sortableHeaders.forEach((header) => {
            header.addEventListener("click", () => {
                const headerKey = header.dataset.sortKey as SortKey | undefined;
                if (!headerKey) {
                    return;
                }
                cycleSort(headerKey);
            });
        });
        DOM.preview.className = "filesPreview";
        document.body.appendChild(DOM.preview);
        window.addEventListener("scroll", hidePreview, true);
        window.addEventListener("filesUploaded", () => {
            loadFiles().then();
        });
        DOM.modalFileRename.addEventListener("hidden.bs.modal", resetRenameModal);
        DOM.modalFileRenameSubmit.addEventListener("click", () => {
            handleRename().then();
        });
        DOM.modalFileRenameBaseName.addEventListener("keydown", (event) => {
            if (event.key === "Enter") {
                event.preventDefault();
                handleRename().then();
            }
        });
        renderFilesAvatar().then();
        loadFiles().then();
    }
})();
