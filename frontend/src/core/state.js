import {ClearTransferHistory, GetTransferHistories} from "../../wailsjs/go/app/App";


const transferHistoryBtn = document.getElementById("history-toggle");
const transferHistoryModal = document.getElementById("transfer-history-modal");
const closeTransferHistoryModal = document.getElementById("close-transfer-history");
const TransferHistoryList = document.getElementById("transfer-history-list");

const clearHistoryBtn = document.getElementById("clear-history-btn");

async function loadTransferHistory() {
    //console.log("Loading transfer History")
    TransferHistoryList.innerHTML = "";

    const transferHistories = await GetTransferHistories();
    //console.log("Sent File Histories", transferHistories);
    const emptyEl = document.getElementById("history-empty-msg");
    if (emptyEl) emptyEl.hidden = transferHistories.length > 0;

    transferHistories.forEach(transferredFile => {
        const li = document.createElement("li");
        li.classList.add("history-item");
        const isSuccess = (transferredFile.status || "").toUpperCase() === "COMPLETED";
        const statusClass = isSuccess ? "success" : "failed";
        const typeLabel = (transferredFile.transfer_type || "").toUpperCase() === "SENT" ? "Sent" : "Received";

        li.innerHTML = `
            <div class="history-file-icon">
                <span class="material-symbols-outlined">description</span>
            </div>
            <div class="history-main">
                <span class="history-filename">${escapeHtml(transferredFile.file_name || "—")}</span>
                <span class="history-transfer-type">${typeLabel}</span>
                <span class="history-status ${statusClass}">${transferredFile.status || "—"}</span>
            </div>
            <div class="history-meta">
                <span class="history-receiver">${escapeHtml(transferredFile.peer || "—")}</span>
                <span class="history-time">${formatTimestamp(transferredFile.time_stamp)}</span>
            </div>
        `;

        TransferHistoryList.append(li);
    });
}

function escapeHtml(text) {
    const div = document.createElement("div");
    div.textContent = text;
    return div.innerHTML;
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

loadTransferHistory().then(r => {
    console.log("transfer Histories is loaded")
});

transferHistoryBtn.addEventListener("click", () => {
    transferHistoryModal.style.display = "flex";
    loadTransferHistory().then(() => console.log("transfer history load"))
});

closeTransferHistoryModal.addEventListener("click", () => {
    transferHistoryModal.style.display = "none";
});


clearHistoryBtn.addEventListener("click", async () => {
    try {
        TransferHistoryList.innerHTML = "";
        const emptyEl = document.getElementById("history-empty-msg");
        if (emptyEl) emptyEl.hidden = false;
        await ClearTransferHistory();
    } catch (e) {
        console.log("Error while clearing transfer histories")
    }
});