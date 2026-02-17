import {EventsOn} from "../../wailsjs/runtime";
import {DisconnectPeer, PickFiles, PickFolder, SendFileToPeer} from "../../wailsjs/go/app/App";
import {showToast} from "../utils/utils";

const connectionErrorMsg = document.getElementById('connection-error-msg');
const connectionErrorModal = document.getElementById('connection-error-modal');
const transferListEl = document.getElementById("transfer-list");
const sendFileCountEl = document.getElementById("send-file-count");
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
    const card = document.createElement("div");
    card.classList.add("transfer-card");
    card.dataset.transferId = transferId;
    card.dataset.peerId = peerId;

    card.innerHTML = `
        <div class="transfer-info">
            <span class="file-label">${fileName}</span>
            <span class="status-label">0%</span>
        </div>
        <div class="progress-container">
            <div class="progress-bar"></div>
        </div>
        <div class="transfer-actions">
            <button class="action-btn pause">⏸</button>
            <button class="action-btn resume" style="display:none;">▶️</button>
            <button class="action-btn cancel">❌</button>
        </div>
    `;

    transferListEl.appendChild(card);

    const transfer = {
        peerId,
        transferId,
        card,
        paused: false,
        state: "active"
    };

    ongoingTransfers.set(id, transfer);
    updateTransferCount();

    const pauseBtn = card.querySelector(".pause");
    const resumeBtn = card.querySelector(".resume");
    const cancelBtn = card.querySelector(".cancel");

    pauseBtn.addEventListener("click", () => {
        if (transfer.state !== "active") return;

        transfer.paused = true;
        pauseBtn.style.display = "none";
        resumeBtn.style.display = "inline-flex";
        // backend pause here
    });

    resumeBtn.addEventListener("click", () => {
        if (transfer.state !== "active") return;

        transfer.paused = false;
        pauseBtn.style.display = "inline-flex";
        resumeBtn.style.display = "none";
        // backend resume here
    });

    cancelBtn.addEventListener("click", () => {
        cancelTransfer(transferId, "user-cancelled");
    });
}


export function cancelTransfer(transferId, reason = "unknown") {
    const transfer = ongoingTransfers.get(transferId);
    if (!transfer) return;

    transfer.state = "failed";

    console.log(`[TRANSFER ENDED] ${transferId} (${reason})`);

    transfer.card.remove();
    ongoingTransfers.delete(transferId);
    failedTransfers.add(transferId);

    updateTransferCount();
    // backend cancel if needed
}


function updateProgress(id, progress) {
    const transfer = ongoingTransfers.get(id);
    if (!transfer || transfer.state !== "active") return;

    const progressBar = transfer.card.querySelector(".progress-bar");
    const statusLabel = transfer.card.querySelector(".status-label");

    progressBar.style.width = `${progress}%`;
    statusLabel.textContent =
        progress === 100
            ? "Completed"
            : `Sending… ${Math.floor(progress)}%`;

    if (progress === 100) {
        setTimeout(() => {
            cancelTransfer(id, "completed");
        }, 1000);
    }
}

sendFileButton.addEventListener("click", async (event) => {
    event.preventDefault();
    try {
        // Open native file dialog via Go
        console.log("filePath", filePath);
        if (filePath.size === 0 || receiverSelect.value === "") {
            alert("Receiver is not provided");
            //console.error("File Path is not provided")
            return
        }

        console.log("receiver ", receiverSelect.value, "filePath ", filePath);
        // Send file to selected peer
        const receiverId = receiverSelect.value;
        SendFileToPeer(receiverId, Array.from(filePath)).then(() => {
            console.log("File sent successfully");
        }).catch(err => {
            showConnectionError("Connection lost! File transfer failed.");
            console.error("Error while sending the file", err);
        });
        resetFileSelection();
        //console.log("File sent:", filePath);
    } catch (err) {
        console.error("Error while sending file:", err);
        showConnectionError("Connection lost! File transfer failed.");
    }
});


function updateTransferCount() {
    console.log("Active transfers:", ongoingTransfers.size);
    sendFileCountEl.textContent = ongoingTransfers.size.toString();
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
        "File transfer failed",
        `Receiver disconnected while sending "${payload.file}"`
    );
});

EventsOn("wails:file-drop", (x, y, paths) => {
    console.log("DROP EVENT FIRED");
    console.log("Dropped files at: ", x, y);
    console.log("File paths: ", paths)

    if (paths && paths.length > 0) {
        console.log("File received via Drag & Drop");
        handleFileSelection(paths);
    }
})



function showConnectionError(message) {
    connectionErrorMsg.textContent = message;
    connectionErrorModal.style.display = 'flex';
}


closeErrorBtn.addEventListener('click', () => {
    connectionErrorModal.style.display = 'none';

    failedTransfers.forEach(id => {
        const transfer = ongoingTransfers.get(id);
        if (transfer) {
            transfer.card.remove();
            ongoingTransfers.delete(id);
        }
    });

    failedTransfers.clear();

    updateTransferCount();
});


function getFileName(path) {
    console.log("Path : ", path)
    return path.split(/[/\\]/).pop();
}

function handleFileSelection(paths) {
    if (paths.length === 0) {
        resetFileSelection();
        return;
    }

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

    console.log("Here in render selected files");
    let i = 0;
    for (const file of filePath) {

        const div = document.createElement("div");
        div.classList.add("file-item");

        div.innerHTML = `<span class="file-index">${i + 1}.</span>
                        <span class="file-name">${getFileName(file)}</span>
                        <div class="file-actions">
                        <button class="remove-btn"> Remove</button>
                        </div>`

        div.querySelector(".remove-btn").addEventListener('click', () => {
            div.remove();
            filePath.delete(file);
            updateUploadFileButtons();
            renderSelectedFiles(filePath)
        });

        selectedFileList.appendChild(div);
        i++;
    }
}

function updateUploadFileButtons() {
    if(filePath.size === 0) {
        sendFileButton.disabled = true;
    }
}


function resetFileSelection() {
    filePath.clear();
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

    document.getElementById("selected-peer-name").textContent = "No Peer Selected";
    document.getElementById("selected-peer-status").textContent = "Selected a Peer from the sidebar";

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
    menu.style.display = menu.style.display === 'flex' ? 'none' : 'flex';
});

// Hide dropdown if clicking outside
document.addEventListener('click', () => {
    menu.style.display = 'none';
});

function showNotification(message, type = 'success') {
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');

    toast.className = `toast ${type}`;
    toast.textContent = message;

    container.appendChild(toast);

    // 1. Wait for 5 seconds of visibility
    setTimeout(() => {
        // 2. Start the fade out
        toast.style.transition = 'opacity 0.5s ease, transform 0.5s ease';
        toast.style.opacity = '0';
        toast.style.transform = 'translateY(-20px)'; // Optional: slight slide up effect

        // 3. Remove from DOM exactly when the 0.5s transition ends
        setTimeout(() => {
            toast.remove();
        }, 500);
    }, 5000);
}


EventsOn("transfer-permission-denied", (payload) => {
    console.log("Here transfer permission in frontend")
    showNotification("Permission denied by the receiver for transfer", "failed");
});