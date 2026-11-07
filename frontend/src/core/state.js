import {ClearTransferHistory, GetTransferHistories} from "../../wailsjs/go/app/App";
import {escapeHtml, fileTypeOf, showToast} from "../utils/utils";
import {closeModal, onModalDismiss, openModal} from "./modal";

const MODAL_ID = "transfer-history-modal";
const historyToggle = document.getElementById("history-toggle");
const closeBtn = document.getElementById("close-transfer-history");
const listEl = document.getElementById("transfer-history-list");
const emptyEl = document.getElementById("history-empty-msg");
const emptyTitle = document.getElementById("history-empty-title");
const emptyHint = document.getElementById("history-empty-hint");
const summaryEl = document.getElementById("history-summary");
const searchEl = document.getElementById("history-search");
const filterTabs = document.querySelectorAll("[data-history-filter]");
const clearBtn = document.getElementById("clear-history-btn");
const confirmBar = document.getElementById("history-confirm");
const confirmCancel = document.getElementById("history-confirm-cancel");
const confirmDelete = document.getElementById("history-confirm-delete");

let histories = [];
let filter = "all";
let query = "";

async function loadTransferHistory() {
    try {
        histories = (await GetTransferHistories()) || [];
    } catch (e) {
        console.error("Failed to load transfer history", e);
        histories = [];
    }
    render();
}

function isSent(item) {
    return (item.transfer_type || "").toUpperCase() === "SENT";
}

function isCompleted(item) {
    return (item.status || "").toUpperCase() === "COMPLETED";
}

function dayKey(ms) {
    const d = new Date(ms);
    return `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`;
}

function dayLabel(ms) {
    const d = new Date(ms);
    const today = new Date();
    const yesterday = new Date();
    yesterday.setDate(today.getDate() - 1);
    if (dayKey(ms) === dayKey(today)) return "Today";
    if (dayKey(ms) === dayKey(yesterday)) return "Yesterday";
    return d.toLocaleDateString(undefined, {
        weekday: "short",
        month: "short",
        day: "numeric",
        year: d.getFullYear() === today.getFullYear() ? undefined : "numeric",
    });
}

function timeLabel(ms) {
    return new Date(ms).toLocaleTimeString(undefined, {hour: "2-digit", minute: "2-digit"});
}

function rowHtml(item) {
    const sent = isSent(item);
    const ok = isCompleted(item);
    const type = fileTypeOf(item.file_name);
    const peer = escapeHtml(item.peer || "Unknown device");
    return `
        <div class="history-row">
            <span class="file-thumb tone-${type.tone}" aria-hidden="true">
                <span class="material-symbols-outlined">${type.icon}</span>
            </span>
            <div class="history-main">
                <span class="history-filename" title="${escapeHtml(item.file_name)}">${escapeHtml(item.file_name || "—")}</span>
                <span class="history-meta">
                    <span class="dir-tag ${sent ? "dir-send" : "dir-receive"}">
                        <span class="material-symbols-outlined">${sent ? "arrow_upward" : "arrow_downward"}</span>
                        ${sent ? "Sent to" : "Received from"}
                    </span>
                    <span class="history-peer">${peer}</span>
                </span>
            </div>
            <div class="history-side">
                <span class="history-time">${timeLabel(item.time_stamp)}</span>
                <span class="status-dot ${ok ? "is-ok" : "is-failed"}">${ok ? "Completed" : "Failed"}</span>
            </div>
        </div>`;
}

function render() {
    const q = query.trim().toLowerCase();
    const visible = histories
        .filter(h => filter === "all" || (filter === "SENT") === isSent(h))
        .filter(h => !q
            || (h.file_name || "").toLowerCase().includes(q)
            || (h.peer || "").toLowerCase().includes(q))
        .sort((a, b) => b.time_stamp - a.time_stamp);

    let html = "";
    let currentDay = null;
    for (const item of visible) {
        const key = dayKey(item.time_stamp);
        if (key !== currentDay) {
            if (currentDay !== null) html += `</div>`;
            html += `<h3 class="history-day">${dayLabel(item.time_stamp)}</h3><div class="history-group">`;
            currentDay = key;
        }
        html += rowHtml(item);
    }
    if (currentDay !== null) html += `</div>`;
    listEl.innerHTML = html;

    const total = histories.length;
    const sentCount = histories.filter(isSent).length;
    summaryEl.textContent = total === 0
        ? "Past sends and receives"
        : `${total} transfer${total === 1 ? "" : "s"} · ${sentCount} sent · ${total - sentCount} received`;

    emptyEl.hidden = visible.length > 0;
    if (total === 0) {
        emptyTitle.textContent = "No transfer history yet";
        emptyHint.textContent = "Files you send and receive will be listed here.";
    } else {
        emptyTitle.textContent = "No matches";
        emptyHint.textContent = q ? `Nothing matches "${query.trim()}".` : "Nothing in this view yet.";
    }
    clearBtn.disabled = total === 0;
}

function hideConfirm() {
    confirmBar.hidden = true;
}

historyToggle.addEventListener("click", () => {
    hideConfirm();
    openModal(MODAL_ID, {focus: "#history-search"});
    loadTransferHistory();
});

closeBtn.addEventListener("click", () => closeModal(MODAL_ID));

onModalDismiss(MODAL_ID, () => {
    // Esc first closes the confirm bar, then the modal
    if (!confirmBar.hidden) hideConfirm();
    else closeModal(MODAL_ID);
});

searchEl.addEventListener("input", () => {
    query = searchEl.value;
    render();
});

filterTabs.forEach(tab => {
    tab.addEventListener("click", () => {
        filterTabs.forEach(t => {
            t.classList.toggle("active", t === tab);
            t.setAttribute("aria-selected", String(t === tab));
        });
        filter = tab.dataset.historyFilter;
        render();
    });
});

clearBtn.addEventListener("click", () => {
    confirmBar.hidden = false;
    confirmCancel.focus();
});

confirmCancel.addEventListener("click", hideConfirm);

confirmDelete.addEventListener("click", async () => {
    try {
        await ClearTransferHistory();
        histories = [];
        render();
        showToast("History cleared", "", "success");
    } catch (e) {
        console.error("Error while clearing transfer histories", e);
        showToast("Couldn't clear history", "Please try again.", "error");
    }
    hideConfirm();
});

loadTransferHistory();
