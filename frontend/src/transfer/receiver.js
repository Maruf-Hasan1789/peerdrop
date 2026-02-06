import {EventsOn} from "../../wailsjs/runtime";
import {showToast} from "../utils/utils";
const receiveListEl = document.getElementById("receive-list");
const receiveCountEl = document.getElementById("receive-count");
const receivingTransfers = new Map();

// --- Events from backend ---

EventsOn("receiving-started", (payload) => {
    // payload: { id, fileName, totalChunks, totalReceived }
    const key = payload.id;
    createReceiveCard(key, payload.file);
});

EventsOn("receiving-progress", (payload) => {
    // payload: { id, fileName, totalChunks, totalReceived }
    const key = payload.id;

    const received = Number(payload.totalReceived) || 0;
    const total = Number(payload.totalChunks) || 1;
    const progress = (received / total) * 100;

    updateReceiveProgress(key, progress);
});

EventsOn("file-received", (payload) => {
    // payload: { id, fileName }
    const key = payload.id;

    if (receivingTransfers.has(key)) {
        updateReceiveProgress(key, 100);

        // Optional: auto-remove after completion
        setTimeout(() => {
            removeReceiveCard(key);
        }, 1000);
    }
});

EventsOn("receiving-failed", (payload) => {
    // payload: { peerId, fileId, fileName }
    console.log("Receiving Failed", payload);

    const key = payload.fileId;
    removeReceiveCard(key);

    showToast(
        `File transfer failed`,
        `Sender disconnected while receiving "${payload.fileName}"`
    );
});

// --- UI creators & helpers ---

function createReceiveCard(key, fileName) {
    if (receivingTransfers.has(key)) return;

    const card = document.createElement("div");
    card.classList.add("transfer-card");
    card.dataset.key = key;

    card.innerHTML = `
        <div class="transfer-info">
            <span class="file-label">${fileName}</span>
            <span class="status-label">Receiving… 0%</span>
        </div>
        <div class="progress-container">
            <div class="progress-bar" style="width: 0"></div>
        </div>
    `;

    if (receiveListEl) receiveListEl.appendChild(card);

    receivingTransfers.set(key, {
        el: card,
        bar: card.querySelector(".progress-bar"),
        label: card.querySelector(".status-label"),
        lastProgress: 0
    });

    updateReceiveCount();
}

function updateReceiveProgress(key, progress) {
    const transfer = receivingTransfers.get(key);
    if (!transfer) return;

    transfer.lastProgress = progress;
    transfer.bar.style.width = `${progress}%`;
    transfer.label.textContent =
        progress >= 100
            ? "Completed"
            : `Receiving… ${Math.floor(progress)}%`;
}

function removeReceiveCard(key) {
    const transfer = receivingTransfers.get(key);
    if (!transfer) return;

    transfer.el.remove();
    receivingTransfers.delete(key);
    updateReceiveCount();
}

function updateReceiveCount() {
    if (receiveCountEl) {
        receiveCountEl.textContent = receivingTransfers.size.toString();
    }
}