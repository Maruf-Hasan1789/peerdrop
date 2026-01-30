console.log("🔥 frontend main.js loaded");
import './style.css'

import {
    ListPeers,
    SendFileToPeer,
    PickFile,
    GetSettings,
    SaveSettings,
    PickDownloadFolder,
    GetTransferHistories,
    HandshakePermission,
    ClearTransferHistory
} from '../wailsjs/go/app/App';
import { EventsOn } from "../wailsjs/runtime";


const peerListEl = document.getElementById("peer-list");
const refreshBtn = document.getElementById("refresh-connected-peer-btn");
const sendFileButton = document.getElementById("send-btn");
const pickFile = document.getElementById("pick-file");
const fileName = document.getElementById("file-name");
const clearSelection = document.getElementById("clear-selection");
let filePath = "";
const receiverSelect = document.getElementById("receiver-select");
const settingsModal = document.getElementById("settings-modal");
const settingsButton = document.getElementById("settings-toggle");
const closeSettingsButton = document.getElementById("close-settings");
const saveSettingsBtn = document.getElementById("saveSettingsButton");
const userName = document.getElementById("userName");
const downloadPath = document.getElementById("downloadPath");
const folderPicker = document.getElementById("folderPicker");
const cancelSettingsBtn = document.getElementById("settingsCancelButton");
let originalSettings = {};
const transferListEl = document.getElementById("transfer-list");
const transferCountEl = document.getElementById("transfer-count");
const transferHistoryBtn = document.getElementById("history-toggle");
const transferHistoryModal = document.getElementById("transfer-history-modal");
const closeTransferHistoryModal = document.getElementById("close-transfer-history");
const TransferHistoryList = document.getElementById("transfer-history-list");
const permissionRequired = document.getElementById('permissionToggle');
const connectionErrorModal = document.getElementById('connection-error-modal');
const closeErrorBtn = document.getElementById('close-error');
const connectionErrorMsg = document.getElementById('connection-error-msg');
const receiveListEl = document.getElementById("receive-list");
const receiveCountEl = document.getElementById("receive-count");
const clearHistoryBtn = document.getElementById("clear-history-btn");
const fileInfoBar = document.getElementById("file-info-bar");
const dropZone = document.getElementById("drop-zone");
const clearFileBtn = document.getElementById("clear-file");



const receivingTransfers = new Map();
// Keep track of ongoing transfers
const ongoingTransfers = new Map();
const failedTransfers = new Set();


async function loadSettings() {
    try {
        const settings = await GetSettings(); // Call Go backend
        //console.log('Loaded settings:', settings);
        originalSettings = { ...settings }

        // Populate inputs
        userName.value = settings.user_name || '';
        downloadPath.value = settings.download_path || '/home/user/Downloads';
        permissionRequired.checked = settings.is_permission_required_to_send_files;

    } catch (err) {
        console.error("Failed to load settings:", err);
    }
}

loadSettings().then(r =>
    console.log("Settings is loaded")
);

loadTransferHistory().then(r => {
    console.log("Transfer Histories is loaded")
});

// Local Map to store peers
const peersMap = new Map();

// Fetch peers from backend
async function fetchPeers() {
    try {
        const peers = await ListPeers();
        console.log("Peer From Backend " + peers)
        peersMap.clear();
        peers.forEach(peer => peersMap.set(peer.id, peer));
        updatePeerListUI();
    } catch (err) {
        console.error("Failed to fetch peers:", err);
    }
}

// Update the DOM

function updatePeerListUI() {
    console.log("Updating Peer List...");

    // Clear the list and the dropdown
    peerListEl.innerHTML = '';

    peersMap.forEach(peer => {
        // --- Right sidebar list ---
        const li = document.createElement("li");
        //li.textContent = `${peer.name} (${peer.id}) - Port: ${peer.port}`;
        li.classList.add("peer-item");
        li.dataset.peerId = peer.id;

        console.log("Peer " + peer.id, peer.name, peer.user_name)

        li.innerHTML = `<strong>${peer.user_name}</strong><br>
                        <small>Port ${peer.port}</small>`

        // Click to highlight in sidebar
        li.addEventListener("click", () => {
            document.querySelectorAll("#peer-list li").forEach(el => el.classList.remove("selected"));
            li.classList.add("selected");

            renderSelectedPeer(peer)
        });

        peerListEl.appendChild(li);
    });
}

async function loadTransferHistory() {
    console.log("Loading Transfer History")
    TransferHistoryList.innerHTML = "";

    const transferHistories = await GetTransferHistories();
    console.log("Sent File Histories", transferHistories);
    transferHistories.forEach(transferredFile => {
        const li = document.createElement("li");
        li.classList.add("history-item");

        li.innerHTML = `
            <div class="history-main">
                <span class="history-filename">${transferredFile.file_name}</span>
                <span class="history-status">${transferredFile.status}</span>
                <span class="history-transfer-type">${transferredFile.transfer_type}</span>
            </div>
        
            <div class="history-meta">
                <span class="history-receiver">
                    to: ${transferredFile.peer}
                </span>
                <span class="history-time">
                    ${formatTimestamp(transferredFile.time_stamp)}
                </span>
            </div>
        `;

        TransferHistoryList.append(li);
    });
}

function formatTimestamp(unixMilliSeconds) {
    const date = new Date(unixMilliSeconds);
    return date.toLocaleString(undefined, {
        year: "numeric",
        month: "short",
        day: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
        hour12: false,
    });
}

document.addEventListener("DOMContentLoaded", () => {
    refreshBtn?.addEventListener("click", fetchPeers);
});



window.addEventListener('dragover', (e) => e.preventDefault());
window.addEventListener('drop', (e) => e.preventDefault());
EventsOn("peer-connected", (peer) => {
    console.log("Peer Added from frontend")
    peersMap.set(peer.id, peer);
    updatePeerListUI();
    console.log(`[CONNECTED] ${peer.user_name} (${peer.id}) : ${peer.port}`);
});

EventsOn("peer-disconnected", (peer) => {
    peersMap.delete(peer.id);
    fetchPeers().then(r => console.log("fetching peers after disconnection"))
    console.log(`[DISCONNECTED] ${peer.user_name} (${peer.id})`);
});



EventsOn("randomEvent", (data) => {
    console.log("random event from frontend " + data)
});


EventsOn("wails:file-drop", (x, y, paths) => {
    console.log("Dropped files at: ", x, y);
    console.log("File paths: ", paths)

    if (paths && paths.length > 0) {
        console.log("File received via Drag & Drop");
        handleFileSelection(paths[0]);
    }
})

sendFileButton.addEventListener("click", async (event) => {
    event.preventDefault();
    console.log("Send Button is selected");
    try {
        // Open native file dialog via Go
        console.log("filePath", filePath);
        if (filePath === "") {
            console.error("File Path is not provided")
            return
        }

        console.log("receiver ", receiverSelect.value, "filePath");
        // Send file to selected peer
        await SendFileToPeer(receiverSelect.value, filePath);
        resetFileSelection();
        console.log("File sent:", filePath);
    } catch (err) {
        console.error("Error while sending file:", err);
        showConnectionError("Connection lost! File transfer failed.");
    }
});

pickFile.addEventListener("click", async () => {
    console.log("Picking File")
    filePath = await PickFile();
    handleFileSelection(filePath);
});

function getFileName(path) {
    return path.split(/[/\\]/).pop();
}

function handleFileSelection(path) {
    if (!path || path === "") {
        resetFileSelection();
        return;
    }

    filePath = path;
    fileName.textContent = getFileName(path);
    // You might want to show file size if available, but for now just name

    sendFileButton.disabled = false;

    // Toggle UI: Keep drop zone visible, show file info bar
    if (dropZone) dropZone.style.display = 'flex';
    if (fileInfoBar) fileInfoBar.style.display = 'flex';

    console.log("File Ready to send", filePath);
}

function resetFileSelection() {
    filePath = "";
    fileName.textContent = "No file selected";
    sendFileButton.disabled = true;
    if (receiverSelect) receiverSelect.value = ""; // Optional: keep receiver?

    // Toggle UI back
    if (dropZone) dropZone.style.display = 'flex';
    if (fileInfoBar) fileInfoBar.style.display = 'none';
}

if (clearFileBtn) {
    clearFileBtn.addEventListener("click", () => {
        resetFileSelection();
    });
}

console.log("Hello");
updatePeerListUI();


function renderSelectedPeer(peer) {
    receiverSelect.value = peer.id;

    document.getElementById("selected-peer-name").textContent = peer.user_name
    document.getElementById("selected-peer-status").textContent = `Port ${peer.port} ${peer.id}`;

    // if (peer.avatar) {
    // document.getElementById("selected-peer-image").src = peer.avatar;
    //  }

    const card = document.getElementById("selected-peer-card")
    card.classList.add("active");
    document.getElementById("clear-selection").style.display = "flex";
}


function hasSettingsChanged() {
    const currentSettings = {
        user_name: userName.value,
        download_path: downloadPath.value,
        is_permission_required_to_send_files: permissionRequired.checked
    };

    return Object.keys(currentSettings).some(key => currentSettings[key] !== originalSettings[key]);
}

clearSelection.addEventListener(("click"), () => {
    document.getElementById("receiver-select").value = "";

    document.getElementById("selected-peer-name").textContent = "No Peer Selected";
    document.getElementById("selected-peer-status").textContent = "Selected a Peer from the sidebar";

    document.querySelectorAll("#peer-list li")
        .forEach(el => el.classList.remove("selected"));
});


settingsButton.addEventListener("click", () => {
    settingsModal.style.display = "flex";
});

closeSettingsButton.addEventListener("click", () => {
    settingsModal.style.display = "none";
});



saveSettingsBtn.addEventListener("click", async () => {
    const updatedSettings = {
        user_name: userName.value,
        download_path: downloadPath.value,
        is_permission_required_to_send_files: permissionRequired.checked
    };

    console.log("Updated Settings", updatedSettings)

    try {
        console.log(updatedSettings)
        await SaveSettings(updatedSettings)
        originalSettings = { ...updatedSettings }
        alert("Settings saved successfully")
        loadSettings().then(r => console.log("Settings loaded"))
    } catch (err) {
        console.error("Failed to save settings")
    }
});



folderPicker.addEventListener("click", async () => {
    try {
        downloadPath.value = await PickDownloadFolder();
    } catch (e) {
        console.log("Error while setting download directory")
        alert("Error while setting download directory")
    }
});

cancelSettingsBtn.addEventListener("click", () => {
    console.log("Original Settings", originalSettings)
    userName.value = originalSettings.user_name;
    downloadPath.value = originalSettings.download_path;
    permissionRequired.checked = originalSettings.is_permission_required_to_send_files;
    settingsModal.style.display = "none";
});

// --- Function to create a new transfer card ---
function createTransferCard(id, fileName) {
    const card = document.createElement("div");
    card.classList.add("transfer-card");
    card.dataset.id = id;

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
            <button class="action-btn cancel" style="display:none;">❌</button>
        </div>
    `;

    // Add card to DOM
    transferListEl.appendChild(card);
    updateTransferCount();

    // Button events
    const pauseBtn = card.querySelector(".pause");
    const resumeBtn = card.querySelector(".resume");
    const cancelBtn = card.querySelector(".cancel");

    let paused = false;

    pauseBtn.addEventListener("click", () => {
        paused = true;
        pauseBtn.style.display = "none";
        resumeBtn.style.display = "inline-flex";
        // Optionally, tell backend to pause
    });

    resumeBtn.addEventListener("click", () => {
        paused = false;
        pauseBtn.style.display = "inline-flex";
        resumeBtn.style.display = "none";
        // Optionally, tell backend to resume
    });

    cancelBtn.addEventListener("click", () => {
        card.remove();
        ongoingTransfers.delete(id);
        updateTransferCount();
        // Optionally, tell backend to cancel
    });

    // Save reference
    ongoingTransfers.set(id, { card, paused });
}

// --- Function to update progress ---
function updateProgress(id, progress) {
    const transfer = ongoingTransfers.get(id);
    if (!transfer) return;

    const progressBar = transfer.card.querySelector(".progress-bar");
    const statusLabel = transfer.card.querySelector(".status-label");

    progressBar.style.width = `${progress}%`;
    statusLabel.textContent = progress === 100 ? "Completed" : `Sending… ${Math.floor(progress)}%`;

    if (progress === 100) {
        setTimeout(() => {
            transfer.card.remove();
            ongoingTransfers.delete(id);
            updateTransferCount();
        }, 1000);
    }
}

// --- Update transfer count ---
function updateTransferCount() {
    transferCountEl.textContent = ongoingTransfers.size.toString();
}

// --- Event Listeners from backend ---
EventsOn("transfer-start", (payload) => {
    console.log(payload);
    createTransferCard(payload.id, payload.fileName);
});



EventsOn("transfer-progress", (payload) => {
    console.log(payload);
    let progress = ((Number(payload.chunkIndex) + 1) / (Number(payload.totalChunks))) * 100
    updateProgress(payload.id, progress);
});

EventsOn("transfer-complete", (payload) => {
    console.log(payload);
    updateProgress(payload.id, 100);
});


EventsOn("transfer-failed", (payload) => {
    console.log(payload);
    failedTransfers.add(payload.id);
});

EventsOn("permission-request", (senderInfo) => {
    console.log(senderInfo);
    showPermissionPopup(senderInfo)
});



function showPermissionPopup(sender) {
    const modal = document.getElementById("permission-modal");
    const message = modal.querySelector(".message");
    const fileList = modal.querySelector(".pm-files");

    message.textContent = `${sender.user_name} wants to send you the following files:`;

    fileList.innerHTML = "";
    sender.files.forEach(file => {
        const li = document.createElement("li");
        li.textContent = file;
        fileList.appendChild(li);
    });

    modal.style.display = "flex";

    document.getElementById("allow-btn").onclick = () => {
        HandshakePermission(sender.id, true).then(r => console.log("Permission granted"));
        modal.style.display = "none";
    };

    document.getElementById("deny-btn").onclick = () => {
        HandshakePermission(sender.id, false).then(r => console.log("Permission denied"));
        modal.style.display = "none";
    };
}



transferHistoryBtn.addEventListener("click", () => {
    transferHistoryModal.style.display = "flex";
    loadTransferHistory().then(() => console.log("transfer history load"))
});

closeTransferHistoryModal.addEventListener("click", () => {
    transferHistoryModal.style.display = "none";
});

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



// --- State ---
const receiveTimeouts = new Map();


// --- Events from backend ---
EventsOn("receiving-started", (payload) => {
    // payload: { id, file, totalChunks, totalReceived }
    // We use payload.file (fileName) as the unique key because ID is unstable
    createReceiveCard(payload.file, payload.file);
});

EventsOn("receiving-progress", (payload) => {
    // payload: { id, file, totalChunks, totalReceived }
    const fileKey = payload.file;
    const received = Number(payload.totalReceived) || 0;
    const total = Number(payload.totalChunks) || 1;

    const progress = (received / total) * 100;
    updateReceiveProgress(fileKey, progress);
});

EventsOn("file-received", (payload) => {
    // payload: { id, file }
    const fileKey = payload.file;
    if (receivingTransfers.has(fileKey)) {
        updateReceiveProgress(fileKey, 100);
    }
});

// --- UI creators ---
function createReceiveCard(key, fileName) {
    if (receivingTransfers.has(key)) return;

    // 1. Set placeholder state
    receivingTransfers.set(key, { loading: true, lastProgress: 0 });

    // 2. Build the DOM element
    const card = document.createElement("div");
    card.classList.add("transfer-card");
    // We can still store the key in dataset if needed
    card.dataset.key = key;

    card.innerHTML = `
        <div class="transfer-info">
            <span class="file-label">${fileName}</span>
            <span class="status-label">Receiving… 0%</span>
        </div>
        <div class="progress-container">
            <div class="progress-bar" style="width: 0%"></div>
        </div>
    `;

    // 3. Append to the correct list
    if (receiveListEl) {
        receiveListEl.appendChild(card);
    }

    // 4. Cache the references for performance
    const transferData = {
        loading: false,
        el: card,
        bar: card.querySelector(".progress-bar"),
        label: card.querySelector(".status-label"),
        lastProgress: 0
    };

    // 5. Check if a progress event arrived while we were building the card
    const pending = receivingTransfers.get(key);
    const initialProgress = (pending && pending.loading) ? pending.lastProgress : 0;

    receivingTransfers.set(key, transferData);

    // Apply any missed progress immediately
    if (initialProgress > 0) {
        updateReceiveProgress(key, initialProgress);
    }

    updateReceiveCount();
}

// --- Progress updates ---
function updateReceiveProgress(key, progress) {
    const data = receivingTransfers.get(key);
    if (!data) return;

    // If the card is still being created, save progress for later
    if (data.loading) {
        data.lastProgress = progress;
        return;
    }

    const displayProgress = Math.min(Math.floor(progress), 100);

    // Prevent redundant DOM updates
    if (displayProgress === data.lastProgress && displayProgress < 100) return;
    data.lastProgress = displayProgress;

    // Use requestAnimationFrame for a smooth visual update
    requestAnimationFrame(() => {
        if (data.bar) {
            data.bar.style.width = `${displayProgress}%`;
        }

        if (data.label) {
            if (displayProgress >= 100) {
                data.label.textContent = "Received";
                scheduleCardRemoval(key);
            } else {
                data.label.textContent = `Receiving… ${displayProgress}%`;
            }
        }
    });
}

// --- Cleanup ---
function scheduleCardRemoval(key) {
    if (receiveTimeouts.has(key)) return;

    const timeout = setTimeout(() => {
        const data = receivingTransfers.get(key);
        if (data && data.el) {
            data.el.remove();
        }
        receivingTransfers.delete(key);
        receiveTimeouts.delete(key);
        updateReceiveCount();
    }, 1500);

    receiveTimeouts.set(key, timeout);
}

// --- Count badge ---
function updateReceiveCount() {
    if (receiveCountEl) {
        receiveCountEl.textContent = receivingTransfers.size.toString();
    }
}

clearHistoryBtn.addEventListener("click", async () => {
    try {
        TransferHistoryList.innerHTML = "";
        await ClearTransferHistory();
    } catch (e) {
        console.log("Error while clearing transfer histories")
    }
});