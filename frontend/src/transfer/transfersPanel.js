import {escapeHtml, fileTypeOf} from "../utils/utils";

const body = document.getElementById("transfers-body");
const sendList = document.getElementById("transfer-list");
const receiveList = document.getElementById("receive-list");
const failedList = document.getElementById("failed-list");
const emptyEl = document.getElementById("transfers-empty");
const emptyTitle = document.getElementById("transfers-empty-title");
const emptyHint = document.getElementById("transfers-empty-hint");
const allCount = document.getElementById("all-count");
const sendCount = document.getElementById("send-file-count");
const receiveCount = document.getElementById("receive-count");
const failedCount = document.getElementById("failed-count");
const tabs = document.querySelectorAll(".transfers-panel .tab");

const EMPTY_COPY = {
    all: ["No transfers yet", "Files you send or receive will show up here."],
    send: ["Nothing sending", "Pick a device, add files and press Send."],
    receive: ["Nothing incoming", "Files other devices send you will show up here."],
    failed: ["No failed transfers", "Interrupted transfers will be listed here."],
};

const DIR_META = {
    send: {icon: "arrow_upward", label: "Sending"},
    receive: {icon: "arrow_downward", label: "Receiving"},
};

/**
 * Builds a transfer row. Keeps the `.transfer-card`, `.file-label`,
 * `.status-label` and `.progress-bar` hooks used by sender.js / receiver.js.
 */
export function createTransferRow(fileName, direction) {
    const type = fileTypeOf(fileName);
    const meta = DIR_META[direction];
    const row = document.createElement("div");
    row.className = "transfer-card transfer-row";
    row.dataset.dir = direction;
    row.innerHTML = `
        <div class="file-thumb tone-${type.tone}" aria-hidden="true">
            <span class="material-symbols-outlined">${type.icon}</span>
        </div>
        <div class="transfer-main">
            <div class="transfer-line">
                <span class="file-label" title="${escapeHtml(fileName)}">${escapeHtml(fileName)}</span>
                <span class="status-label">0%</span>
            </div>
            <div class="progress-container"><div class="progress-bar" style="width: 0"></div></div>
            <div class="transfer-sub">
                <span class="dir-tag dir-${direction}">
                    <span class="material-symbols-outlined">${meta.icon}</span>${meta.label}
                </span>
                <span class="transfer-detail"></span>
            </div>
        </div>
    `;
    (direction === "send" ? sendList : receiveList).appendChild(row);
    refreshTransfersPanel();
    return row;
}

/** Updates bar + label; shows a "Done" state at 100%. */
export function setRowProgress(row, progress, detail) {
    if (!row) return;
    const pct = Math.max(0, Math.min(100, progress));
    row.querySelector(".progress-bar").style.width = `${pct}%`;
    const label = row.querySelector(".status-label");
    if (pct >= 100) {
        row.classList.add("is-done");
        label.innerHTML = `<span class="material-symbols-outlined">check_circle</span>Done`;
    } else {
        label.textContent = `${Math.floor(pct)}%`;
    }
    if (detail !== undefined) row.querySelector(".transfer-detail").textContent = detail;
}

/** Fades a finished row out, then removes it. */
export function removeRowAnimated(row, onRemoved) {
    if (!row) return;
    row.classList.add("is-leaving");
    setTimeout(() => {
        row.remove();
        onRemoved?.();
        refreshTransfersPanel();
    }, 260);
}

/** Moves a row to the Failed list with a dismiss button. */
export function markRowFailed(row, reason) {
    if (!row) return;
    row.classList.remove("is-done");
    row.classList.add("is-failed");
    row.querySelector(".status-label").innerHTML =
        `<span class="material-symbols-outlined">error</span>Failed`;
    row.querySelector(".transfer-detail").textContent = reason || "";
    if (!row.querySelector(".row-dismiss")) {
        const btn = document.createElement("button");
        btn.type = "button";
        btn.className = "row-dismiss";
        btn.setAttribute("aria-label", "Dismiss");
        btn.innerHTML = `<span class="material-symbols-outlined">close</span>`;
        btn.addEventListener("click", () => removeRowAnimated(row));
        row.appendChild(btn);
    }
    failedList.appendChild(row);
    refreshTransfersPanel();
}

export function refreshTransfersPanel() {
    const nSend = sendList.querySelectorAll(".transfer-row").length;
    const nReceive = receiveList.querySelectorAll(".transfer-row").length;
    const nFailed = failedList.querySelectorAll(".transfer-row").length;

    sendCount.textContent = nSend;
    receiveCount.textContent = nReceive;
    failedCount.textContent = nFailed;
    allCount.textContent = nSend + nReceive + nFailed;
    failedCount.classList.toggle("has-items", nFailed > 0);

    const filter = body.dataset.filter;
    const visible = {
        all: nSend + nReceive + nFailed,
        send: nSend,
        receive: nReceive,
        failed: nFailed,
    }[filter];
    emptyEl.hidden = visible > 0;
    [emptyTitle.textContent, emptyHint.textContent] = EMPTY_COPY[filter];
}

tabs.forEach(tab => {
    tab.addEventListener("click", () => {
        tabs.forEach(t => {
            t.classList.toggle("active", t === tab);
            t.setAttribute("aria-selected", String(t === tab));
        });
        body.dataset.filter = tab.dataset.filter;
        refreshTransfersPanel();
    });
});

refreshTransfersPanel();
