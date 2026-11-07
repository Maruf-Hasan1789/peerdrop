import {EventsOn} from "../../wailsjs/runtime";
import {DisconnectPeer, PickFiles, PickFolder, SendFileToPeer} from "../../wailsjs/go/app/App";
import {escapeHtml, fileTypeOf, showToast} from "../utils/utils";
import {closeModal, onModalDismiss, openModal} from "../core/modal";
import {createTransferRow, markRowFailed, refreshTransfersPanel, removeRowAnimated, setRowProgress} from "./transfersPanel";

const connectionErrorMsg = document.getElementById('connection-error-msg');
const ERROR_MODAL_ID = 'connection-error-modal';
const closeErrorBtn = document.getElementById('close-error');
const sendFileButton = document.getElementById("send-btn");
export const receiverSelect = document.getElementById("receiver-select");
const clearSelection = document.getElementById("clear-selection");



const fileInfoBar = document.getElementById("file-info-bar");
const dropZone = document.getElementById("drop-zone");
const clearFileBtn = document.getElementById("clear-file");
let filePath = new Set();
const selectedFileList = document.getElementById("selectedFileListContainer");


const pickFile = document.getElementById("pick-file");
const pickFolder = document.getElementById("pick-folder");

const toggle = document.getElementById('browse-dropdown-toggle');
const menu = document.getElementById('browse-dropdown-menu');


export const ongoingTransfers = new Map();
// transferId -> { peerId, card, paused }
const failedTransfers = new Set();


function createTransferCard(id, peerId, fileName, transferId) {
    const card = createTransferRow(fileName, "send");
    card.dataset.transferId = transferId;
    card.dataset.peerId = peerId;

    ongoingTransfers.set(id, {
        peerId,
        transferId,
        card,
        paused: false,
        state: "active"
    });
    updateTransferCount();
}


function findTransferKey(key) {
    if (ongoingTransfers.has(key)) return key;
    for (const [k, t] of ongoingTransfers) {
        if (t.transferId === key) return k;
    }
    return undefined;
}

const FAIL_REASONS = {
    "peer-disconnected": "Device disconnected",
    "backend-failed": "Connection lost",
};

export function cancelTransfer(transferId, reason = "unknown") {
    const key = findTransferKey(transferId);
    if (key === undefined) return;
    const transfer = ongoingTransfers.get(key);

    console.log(`[TRANSFER ENDED] ${transferId} (${reason})`);
    ongoingTransfers.delete(key);

    if (reason === "completed") {
        transfer.state = "completed";
        removeRowAnimated(transfer.card);
    } else {
        transfer.state = "failed";
        failedTransfers.add(key);
        markRowFailed(transfer.card, FAIL_REASONS[reason] || "Transfer interrupted");
    }

    updateTransferCount();
}


function updateProgress(id, progress) {
    const transfer = ongoingTransfers.get(id);
    if (!transfer || transfer.state !== "active") return;

    setRowProgress(transfer.card, progress);

    if (progress >= 100) {
        transfer.state = "finishing";
        setTimeout(() => {
            cancelTransfer(id, "completed");
        }, 2500);
    }
}

function initSendButton() {
    const btn = document.getElementById("send-btn");
    if (!btn) return;
    btn.addEventListener("click", async (event) => {
        event.preventDefault();
        event.stopPropagation();
        try {
            if (receiverSelect.value === "") {
                showToast("Choose a device first", "Pick who to send to from Nearby devices.", "info");
                return;
            }
            if (filePath.size === 0) {
                showToast("No files selected", "Drop files here or use Browse.", "info");
                return;
            }

            const receiverId = receiverSelect.value;
            const paths = Array.from(filePath).filter(p => typeof p === "string" && p.trim().length > 0);
            if (paths.length === 0) {
                showToast("Couldn't read those files", "Try selecting them again.", "error");
                return;
            }

            await SendFileToPeer(receiverId, paths);
            resetFileSelection();
        } catch (err) {
            console.error("Error while sending file:", err);
            showConnectionError("The connection to the device was lost before the transfer started. Check that it's still on the same network.");
        }
    });
}

if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initSendButton);
} else {
    initSendButton();
}


function updateTransferCount() {
    refreshTransfersPanel();
}


EventsOn("transfer-start", (payload) => {
    console.log(payload);
    createTransferCard(payload.id, payload.peerId , payload.fileName, payload.transferId);
});


EventsOn("transfer-progress", (payload) => {
    console.log(payload);
    let progress = parseFloat(payload.progress) * 100;
    updateProgress(payload.id, progress);
});

EventsOn("transfer-complete", (payload) => {
    console.log(payload);
    updateProgress(payload.id, 100);
});


EventsOn("transfer-failed", (payload) => {
    console.log(payload);
    cancelTransfer(payload.transferId, payload.reason ?? "backend-failed");
    showToast(
        "Sending failed",
        `The receiver disconnected while sending "${payload.file}".`,
        "error"
    );
});

EventsOn("wails:drop", (x, y, paths) => {
    console.log("DROP EVENT FIRED");
    console.log("Dropped files at: ", x, y);
    console.log("File paths: ", paths)

    if (paths && paths.length > 0) {
        console.log("File received via Drag & Drop");
        handleFileSelection(paths);
    }
})


/*
document.addEventListener("drop", (event) => {
    event.preventDefault();

    const folderPaths = [];

    for (const item of event.dataTransfer.items) {
        const entry = item.webkitGetAsEntry();
        console.log(entry)
        if (entry && entry.isDirectory) {
            folderPaths.push(entry.fullPath || entry.name);
        } else if (entry && entry.isFile) {
            // Optional: if you also want top-level files
            // folderPaths.push(item.getAsFile().path);
            folderPaths.push(entry.fullPath || entry.name)
        }
    }

    if (folderPaths.length > 0) {
        console.log("Top-level folders dropped:", folderPaths);
        handleFileSelection(folderPaths); // send to Wails backend
    }
});

 */

document.addEventListener("dragover", (event) => event.preventDefault());


function showConnectionError(message) {
    connectionErrorMsg.textContent = message;
    openModal(ERROR_MODAL_ID, {focus: "#retry-send"});
}

function dismissConnectionError() {
    closeModal(ERROR_MODAL_ID);

    failedTransfers.forEach(id => {
        const transfer = ongoingTransfers.get(id);
        if (transfer) {
            transfer.card.remove();
            ongoingTransfers.delete(id);
        }
    });

    failedTransfers.clear();

    updateTransferCount();
}

closeErrorBtn.addEventListener('click', dismissConnectionError);
onModalDismiss(ERROR_MODAL_ID, dismissConnectionError);

// The selection is kept when sending fails, so "Try again" just resends it.
document.getElementById('retry-send').addEventListener('click', () => {
    dismissConnectionError();
    sendFileButton.click();
});


function getParentDir(path) {
    const i = Math.max(path.lastIndexOf("/"), path.lastIndexOf("\\"));
    return i > 0 ? path.slice(0, i) : path;
}

function getFileName(path) {
    console.log("Path : ", path)
    return path.split(/[/\\]/).pop();
}

export function handleFileSelection(paths) {
    // A cancelled picker returns nothing: keep the current selection.
    if (!paths || paths.length === 0) return;

    paths.forEach(path => filePath.add(path));

    renderSelectedFiles(filePath);
    console.log("File Path: ", filePath);

    sendFileButton.disabled = false;

    // Toggle UI: Keep drop zone visible, show file info bar
    if (dropZone) dropZone.style.display = 'flex';
    if (fileInfoBar) fileInfoBar.style.display = 'flex';

    console.log("File Ready to send", filePath);
}

function renderSelectedFiles(filePath) {
    selectedFileList.innerHTML = "";

    for (const file of filePath) {
        const name = getFileName(file);
        const type = fileTypeOf(name);
        const div = document.createElement("div");
        div.classList.add("file-item");

        div.innerHTML = `<div class="file-thumb tone-${type.tone}" aria-hidden="true">
                            <span class="material-symbols-outlined">${type.icon}</span>
                        </div>
                        <div class="file-item-text">
                            <span class="file-name" title="${escapeHtml(file)}">${escapeHtml(name)}</span>
                            <span class="file-path">${escapeHtml(getParentDir(file))}</span>
                        </div>
                        <button type="button" class="remove-btn" aria-label="Remove ${escapeHtml(name)}" title="Remove">
                            <span class="material-symbols-outlined">close</span>
                        </button>`;

        div.querySelector(".remove-btn").addEventListener("click", (e) => {
            e.stopPropagation();
            filePath.delete(file);
            updateUploadFileButtons();
            renderSelectedFiles(filePath);
        });

        selectedFileList.appendChild(div);
    }
    updateSendSummary();
}

function updateSendSummary() {
    const n = filePath.size;
    const noun = n === 1 ? "item" : "items";
    const summary = document.getElementById("selected-file-summary");
    const label = document.getElementById("send-btn-label");
    if (summary) summary.textContent = `${n} ${noun} ready to send`;
    if (label) label.textContent = n > 0 ? `Send ${n} ${noun}` : "Send";
}

function updateUploadFileButtons() {
    if (filePath.size === 0) {
        sendFileButton.disabled = true;
        if (dropZone) dropZone.style.display = 'flex';
        if (fileInfoBar) fileInfoBar.style.display = 'none';
    } else {
        sendFileButton.disabled = false;
    }
}


function resetFileSelection() {
    filePath.clear();
    selectedFileList.innerHTML = "";
    updateSendSummary();
    sendFileButton.disabled = true;

    // Toggle UI back
    if (dropZone) dropZone.style.display = 'flex';
    if (fileInfoBar) fileInfoBar.style.display = 'none';
    console.log("File Path: ", filePath);
}

if (clearFileBtn) {
    clearFileBtn.addEventListener("click", () => {
        resetFileSelection();
    });
}

clearSelection.addEventListener(("click"), clearReceiverSelection);

export function clearReceiverSelection() {
    if(receiverSelect.value.length > 0) {
        DisconnectPeer(receiverSelect.value).then(r =>{
            console.log("Receiver Select is disconnected");
        });
    }

    console.log("Receiver Select Value: ", receiverSelect.value);

    document.getElementById("receiver-select").value = "";

    document.getElementById("selected-peer-name").textContent = "No device selected";
    document.getElementById("selected-peer-status").textContent = "Pick one from Nearby devices";
    const avatar = document.getElementById("selected-peer-avatar");
    if (avatar) {
        avatar.removeAttribute("style");
        avatar.innerHTML = `<span class="material-symbols-outlined">devices</span>`;
    }
    const card = document.getElementById("selected-peer-card");
    card.classList.remove("active");
    card.classList.add("empty-state");
    clearSelection.style.display = "none";

    document.querySelectorAll("#peer-list li")
        .forEach(el => el.classList.remove("selected"));
}


pickFile.addEventListener("click", async () => {
    console.log("Picking File")
    const pickedFiles = await PickFiles();
    console.log("Picked Files ", pickedFiles);
    handleFileSelection(pickedFiles);
});

pickFolder.addEventListener("click", async ()=> {
   console.log("Picking Folder");
   const pickedFolder = await PickFolder();
   console.log("Picked Folder ", pickedFolder)
    handleFileSelection(pickedFolder);
});


toggle.addEventListener('click', (e) => {
    e.stopPropagation(); // prevent document click from immediately hiding
    const open = menu.style.display !== 'flex';
    menu.style.display = open ? 'flex' : 'none';
    toggle.classList.toggle('is-open', open);
});

// Hide dropdown if clicking outside
document.addEventListener('click', () => {
    menu.style.display = 'none';
    toggle.classList.remove('is-open');
});

EventsOn("transfer-permission-denied", (payload) => {
    console.log("Here transfer permission in frontend")
    showToast("Transfer declined", "The receiving device didn't accept the files.", "error");
});