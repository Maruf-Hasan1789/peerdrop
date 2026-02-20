import { EventsOn } from "../../wailsjs/runtime";
import { showToast } from "../utils/utils";
import { ReceiveFilePermission } from "../../wailsjs/go/app/App";
const receiveListEl = document.getElementById("receive-list");
const receiveCountEl = document.getElementById("receive-count");
const receivingTransfers = new Map();




EventsOn("permission-request", (senderInfo) => {
    console.log(senderInfo);
    showPermissionPopup(senderInfo)
});

const PermissionMode = {
    None: 0,
    Partial: 1,
    All: 2
};


let permissionState = {
    mode: PermissionMode.None,
    files: {}
};

function showPermissionPopup(sender) {
    const modal = document.getElementById("permission-modal");
    const fileList = modal.querySelector(".pm-files");
    const partialButton = document.getElementById("partial-permission-btn");
    const allowAllBtn = document.getElementById("allow-btn");
    const denyAllBtn = document.getElementById("deny-btn");

    permissionState = {
        mode: PermissionMode.Partial,
        files: {}
    };

    fileList.innerHTML = "";
    partialButton.disabled = true;
    modal.querySelector(".message").textContent =
        `${sender.user_name} wants to send you these files:`;

    sender.files.forEach(file => {
        const fileID = file.id;
        const li = document.createElement("li");
        li.className = "pm-file-item";

        const sizeMB = file.file_size / (1024 * 1024);
        const info = document.createElement("span");
        info.className = "pm-file-info";
        info.textContent = `${file.file_name} · ${decimalNumberFormatter.format(sizeMB)} MB`;

        const btnBox = document.createElement("div");
        btnBox.className = "pm-file-actions";

        const allowBtn = document.createElement("button");
        allowBtn.type = "button";
        allowBtn.textContent = "Allow";
        allowBtn.className = "btn-allow";

        const denyBtn = document.createElement("button");
        denyBtn.type = "button";
        denyBtn.textContent = "Deny";
        denyBtn.className = "btn-deny";

        // --- ROW-LEVEL CLICK HANDLERS ---
        // --- ROW-LEVEL CLICK HANDLERS ---
        allowBtn.onclick = () => {
            // If already allowed, toggle off (reset)
            if (permissionState.files[fileID] === true) {
                delete permissionState.files[fileID];
                updateRowUI(allowBtn, denyBtn, null);
            } else {
                // Set to allowed
                permissionState.files[fileID] = true;
                updateRowUI(allowBtn, denyBtn, true);
            }
            checkPartialButton();
        };

        denyBtn.onclick = () => {
            // If already denied, toggle off (reset)
            if (permissionState.files[fileID] === false) {
                delete permissionState.files[fileID];
                updateRowUI(allowBtn, denyBtn, null);
            } else {
                // Set to denied
                permissionState.files[fileID] = false;
                updateRowUI(allowBtn, denyBtn, false);
            }
            checkPartialButton();
        };

        btnBox.append(allowBtn, denyBtn);
        li.append(info, btnBox);
        fileList.appendChild(li);
    });


    function updateRowUI(allow, deny, isAllowed) {
        if (isAllowed === null) {
            // Reset state: show both
            allow.style.display = "inline-block";
            deny.style.display = "inline-block";

            allow.classList.remove("active");
            deny.classList.remove("active");
        } else if (isAllowed) {
            // Allowed: Show Allow (active), Hide Deny
            allow.style.display = "inline-block";
            deny.style.display = "none";

            allow.classList.add("active");
            deny.classList.remove("active");
        } else {
            // Denied: Hide Allow, Show Deny (active)
            allow.style.display = "none";
            deny.style.display = "inline-block";

            allow.classList.remove("active");
            deny.classList.add("active");
        }
    }

    function checkPartialButton() {
        const count = Object.keys(permissionState.files).length;
        partialButton.disabled = (count === 0);
    }

    function finalizeAndSend(mode) {
        permissionState.mode = mode;

        if (mode !== PermissionMode.Partial) {
            permissionState.files = {};
        } else {
            sender.files.forEach(file => {
                const fid = file.id;
                if (permissionState.files[fid] === undefined) {
                    permissionState.files[fid] = false;
                }
            });
        }

        ReceiveFilePermission(sender.id, sender.transfer_id, permissionState)
            .then(() => {
                console.log(`Permission sent: ${mode}`);
                modal.style.display = "none";
            })
            .catch(err => console.error("Failed to send permission:", err));
    }

    // --- MAIN ACTION BUTTON HANDLERS ---
    // Using .onclick ensures we overwrite previous listeners from other popup calls
    allowAllBtn.onclick = () => finalizeAndSend(PermissionMode.All);
    denyAllBtn.onclick = () => finalizeAndSend(PermissionMode.None);
    partialButton.onclick = () => finalizeAndSend(PermissionMode.Partial);

    // Show the modal
    modal.style.display = "flex";
}





const decimalNumberFormatter = new Intl.NumberFormat('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
});





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

    const progress = (received / total) * 100;

    updateReceiveProgress(key, progress);
});

EventsOn("file-received", (payload) => {
    // payload: { id, file }
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

    const key = payload.Id;
    removeReceiveCard(key);

    showToast(
        `File transfer failed`,
        `Sender disconnected while receiving "${payload.file}"`
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