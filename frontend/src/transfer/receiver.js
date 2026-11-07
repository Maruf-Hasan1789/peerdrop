import { EventsOn, WindowShow, WindowUnminimise } from "../../wailsjs/runtime";
import { escapeHtml, fileTypeOf, formatBytes, hueOf, initialsOf, showToast } from "../utils/utils";
import { closeModal, openModal } from "../core/modal";
import { createTransferRow, markRowFailed, refreshTransfersPanel, removeRowAnimated, setRowProgress } from "./transfersPanel";
import { ReceiveFilePermission } from "../../wailsjs/go/app/App";
const receivingTransfers = new Map();




EventsOn("permission-request", (senderInfo) => {
    console.log(senderInfo);
    showPermissionPopup(senderInfo)
});

const PermissionMode = {
    Unspecified: 0,
    None: 1,
    Partial: 2,
    All: 3
};


function showPermissionPopup(sender) {
    const modal = document.getElementById("permission-modal");
    const fileList = modal.querySelector(".pm-files");
    const selectAll = document.getElementById("pm-select-all");
    const summary = document.getElementById("pm-selected-summary");
    const acceptBtn = document.getElementById("allow-btn");
    const acceptLabel = document.getElementById("allow-btn-label");
    const declineBtn = document.getElementById("deny-btn");
    const files = sender.files || [];
    const senderName = sender.user_name || "A device";
    const totalBytes = files.reduce((sum, f) => sum + Number(f.file_size || 0), 0);
    const plural = (n) => `${n} file${n === 1 ? "" : "s"}`;

    // Header: sender avatar + headline
    const avatar = document.getElementById("permission-avatar");
    avatar.style.setProperty("--avatar-hue", hueOf(sender.id || senderName));
    avatar.textContent = initialsOf(senderName);
    document.getElementById("permission-title").textContent =
        `${senderName} wants to send you ${plural(files.length)}`;
    document.getElementById("permission-message").textContent =
        `${formatBytes(totalBytes)} in total · untick anything you don't want`;

    // One checkbox per file, all ticked by default
    fileList.innerHTML = "";
    files.forEach(file => {
        const type = fileTypeOf(file.file_name);
        const li = document.createElement("li");
        li.className = "pm-file-item";
        li.innerHTML = `
            <label class="pm-file">
                <input type="checkbox" class="checkbox pm-check" data-file-id="${escapeHtml(String(file.id))}" checked>
                <span class="file-thumb tone-${type.tone}" aria-hidden="true">
                    <span class="material-symbols-outlined">${type.icon}</span>
                </span>
                <span class="pm-file-text">
                    <span class="pm-file-name" title="${escapeHtml(file.file_name)}">${escapeHtml(file.file_name)}</span>
                    <span class="pm-file-size">${formatBytes(file.file_size)}</span>
                </span>
            </label>`;
        fileList.appendChild(li);
    });

    const checks = () => [...fileList.querySelectorAll(".pm-check")];

    function refresh() {
        const all = checks();
        const picked = all.filter(c => c.checked);
        const pickedBytes = files
            .filter(f => picked.some(c => c.dataset.fileId === String(f.id)))
            .reduce((sum, f) => sum + Number(f.file_size || 0), 0);
        selectAll.checked = picked.length === all.length && all.length > 0;
        selectAll.indeterminate = picked.length > 0 && picked.length < all.length;
        summary.textContent = `${picked.length} of ${all.length} selected · ${formatBytes(pickedBytes)}`;
        acceptBtn.disabled = picked.length === 0;
        acceptLabel.textContent = picked.length === all.length
            ? (all.length === 1 ? "Accept" : "Accept all")
            : `Accept ${picked.length}`;
    }

    fileList.onchange = refresh;
    selectAll.onchange = () => {
        checks().forEach(c => { c.checked = selectAll.checked; });
        refresh();
    };
    refresh();

    function finalizeAndSend(mode) {
        const state = {mode, files: {}};
        if (mode === PermissionMode.Partial) {
            checks().forEach(c => {
                // ids are numeric or string on the Go side; keep the original type
                const original = files.find(f => String(f.id) === c.dataset.fileId);
                state.files[original ? original.id : c.dataset.fileId] = c.checked;
            });
        }
        acceptBtn.disabled = true;
        declineBtn.disabled = true;
        ReceiveFilePermission(sender.id, sender.transfer_id, state)
            .then(() => closeModal(modal))
            .catch(err => {
                console.error("Failed to send permission:", err);
                showToast("Couldn't answer the request", "The sender may have disconnected.", "error");
                closeModal(modal);
            })
            .finally(() => { declineBtn.disabled = false; refresh(); });
    }

    // .onclick overwrites handlers left by a previous request
    acceptBtn.onclick = () => {
        const all = checks();
        const picked = all.filter(c => c.checked).length;
        if (picked === 0) return;
        finalizeAndSend(picked === all.length ? PermissionMode.All : PermissionMode.Partial);
    };
    declineBtn.onclick = () => finalizeAndSend(PermissionMode.None);

    // Bring the window forward so the request isn't missed
    try {
        WindowUnminimise();
        WindowShow();
    } catch (e) {
        console.warn("Could not focus window", e);
    }
    openModal(modal, {focus: "#allow-btn"});
}

EventsOn("receiving-started", (payload) => {
    // payload: { id, fileName, totalChunks, totalReceived }
    const key = payload.id;
    createReceiveCard(key, payload.file);
});

EventsOn("receiving-progress", (payload) => {
    const key = payload.id;

    const received = Number(payload.totalReceived);
    const total = Number(payload.totalBytes);

    console.log("Received " + received + " Total " + total)

    const progress = total > 0 ? (received / total) * 100 : 0;

    updateReceiveProgress(key, progress, `${formatBytes(received)} of ${formatBytes(total)}`);
});

EventsOn("file-received", (payload) => {
    // payload: { id, file }
    const key = payload.id;

    if (receivingTransfers.has(key)) {
        updateReceiveProgress(key, 100);

        // Keep the "Done" state visible briefly, then remove
        setTimeout(() => {
            removeReceiveCard(key);
        }, 2500);
    }
});

EventsOn("receiving-failed", (payload) => {
    // payload: { peerId, fileId, fileName }
    console.log("Receiving Failed", payload);

    const key = payload.Id ?? payload.id;
    const transfer = receivingTransfers.get(key);
    if (transfer) {
        receivingTransfers.delete(key);
        markRowFailed(transfer.el, "Sender disconnected");
    }

    showToast(
        "Receiving failed",
        `The sender disconnected while sending "${payload.file}".`,
        "error"
    );
});

// --- UI creators & helpers ---

function createReceiveCard(key, fileName) {
    if (receivingTransfers.has(key)) return;

    const card = createTransferRow(fileName, "receive");
    card.dataset.key = key;

    receivingTransfers.set(key, {
        el: card,
        lastProgress: 0
    });

    updateReceiveCount();
}

function updateReceiveProgress(key, progress, detail) {
    const transfer = receivingTransfers.get(key);
    if (!transfer) return;

    transfer.lastProgress = progress;
    setRowProgress(transfer.el, progress, detail);
}

function removeReceiveCard(key) {
    const transfer = receivingTransfers.get(key);
    if (!transfer) return;

    receivingTransfers.delete(key);
    removeRowAnimated(transfer.el);
    updateReceiveCount();
}

function updateReceiveCount() {
    refreshTransfersPanel();
}
