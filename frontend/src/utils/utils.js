
const TOAST_ICONS = {info: "info", success: "check_circle", error: "error"};

/**
 * Shows a stacked notification in the bottom-right corner.
 * @param {string} title
 * @param {string} [message]
 * @param {"info"|"success"|"error"} [type]
 */
export function showToast(title, message = "", type = "info") {
    const container = document.getElementById("toast-container");
    if (!container) return;

    const toast = document.createElement("div");
    toast.className = `toast toast-${type}`;
    toast.setAttribute("role", type === "error" ? "alert" : "status");
    toast.innerHTML = `
        <span class="toast-icon material-symbols-outlined" aria-hidden="true">${TOAST_ICONS[type] || "info"}</span>
        <div class="toast-text">
            <strong>${escapeHtml(title)}</strong>
            ${message ? `<span>${escapeHtml(message)}</span>` : ""}
        </div>
        <button type="button" class="toast-close" aria-label="Dismiss">
            <span class="material-symbols-outlined">close</span>
        </button>
    `;

    let timer;
    const dismiss = () => {
        clearTimeout(timer);
        toast.classList.add("is-leaving");
        setTimeout(() => toast.remove(), 200);
    };
    const arm = () => { timer = setTimeout(dismiss, type === "error" ? 7000 : 4500); };

    toast.querySelector(".toast-close").addEventListener("click", dismiss);
    toast.addEventListener("mouseenter", () => clearTimeout(timer));
    toast.addEventListener("mouseleave", arm);

    container.appendChild(toast);
    // Keep at most 4 on screen.
    while (container.children.length > 4) container.firstElementChild.remove();
    arm();
}

export function escapeHtml(text) {
    const div = document.createElement("div");
    div.textContent = text ?? "";
    return div.innerHTML;
}

const FILE_TYPES = [
    {tone: "video", icon: "movie", ext: ["mp4", "mov", "mkv", "avi", "webm", "m4v"]},
    {tone: "image", icon: "image", ext: ["png", "jpg", "jpeg", "gif", "webp", "heic", "svg", "bmp"]},
    {tone: "audio", icon: "music_note", ext: ["mp3", "wav", "flac", "m4a", "aac", "ogg"]},
    {tone: "archive", icon: "folder_zip", ext: ["zip", "rar", "7z", "tar", "gz", "tgz", "xz", "bz2"]},
    {tone: "doc", icon: "article", ext: ["pdf", "doc", "docx", "txt", "md", "rtf", "pages", "odt"]},
    {tone: "sheet", icon: "table_chart", ext: ["xls", "xlsx", "csv", "numbers", "ods"]},
    {tone: "code", icon: "code", ext: ["js", "ts", "go", "py", "java", "json", "html", "css", "sh", "yaml", "yml", "log"]},
    {tone: "app", icon: "deployed_code", ext: ["exe", "dmg", "pkg", "apk", "deb", "rpm", "appimage", "msi"]},
];

/** Returns a Material Symbols icon name and a colour tone for a file name. */
export function fileTypeOf(fileName) {
    const name = (fileName || "").toLowerCase();
    const dot = name.lastIndexOf(".");
    if (dot <= 0) return {tone: "folder", icon: "folder"};
    const ext = name.slice(dot + 1);
    return FILE_TYPES.find(t => t.ext.includes(ext)) || {tone: "generic", icon: "draft"};
}

export function initialsOf(name) {
    const parts = (name || "?").trim().split(/[\s._-]+/).filter(Boolean);
    if (parts.length === 0) return "?";
    if (parts.length === 1) {
        // Split CamelCase names like "MarufHasan" into "MH"
        const caps = parts[0].match(/[A-Z]/g);
        if (caps && caps.length >= 2) return (caps[0] + caps[1]).toUpperCase();
        return parts[0].slice(0, 2).toUpperCase();
    }
    return (parts[0][0] + parts[1][0]).toUpperCase();
}

/** Stable hue (0–359) derived from a string, used for avatar colours. */
export function hueOf(text) {
    let h = 0;
    for (const ch of text || "") h = (h * 31 + ch.charCodeAt(0)) % 360;
    return h;
}

export function formatBytes(bytes) {
    const n = Number(bytes);
    if (!Number.isFinite(n) || n <= 0) return "0 B";
    const units = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.min(Math.floor(Math.log(n) / Math.log(1024)), units.length - 1);
    const v = n / Math.pow(1024, i);
    return `${v >= 100 || i === 0 ? Math.round(v) : v.toFixed(1)} ${units[i]}`;
}
