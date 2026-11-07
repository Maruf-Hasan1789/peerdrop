import { GetSettings, PickDownloadFolder, SaveSettings } from "../../wailsjs/go/app/App";
import { BrowserOpenURL } from "../../wailsjs/runtime";
import { showToast } from "../utils/utils";
import { closeModal, onModalDismiss, openModal } from "./modal";

const MODAL_ID = "settings-modal";
const settingsButton = document.getElementById("settings-toggle");
const closeSettingsButton = document.getElementById("close-settings");
const saveSettingsBtn = document.getElementById("saveSettingsButton");
const cancelSettingsBtn = document.getElementById("settingsCancelButton");
const userName = document.getElementById("userName");
const downloadPath = document.getElementById("downloadPath");
const folderPicker = document.getElementById("folderPicker");
const permissionRequired = document.getElementById("permissionToggle");
const footer = document.getElementById("settings-footer");
const myDeviceName = document.getElementById("my-device-name");

let originalSettings = {
    user_name: "",
    download_path: "",
    is_permission_required_to_send_files: false
};

function currentSettings() {
    return {
        user_name: userName.value.trim(),
        download_path: downloadPath.value,
        is_permission_required_to_send_files: permissionRequired.checked
    };
}

function fillInputs(settings) {
    userName.value = settings.user_name;
    downloadPath.value = settings.download_path;
    downloadPath.title = settings.download_path;
    permissionRequired.checked = settings.is_permission_required_to_send_files;
    updateSaveButton();
}

function hasSettingsChanged() {
    const current = currentSettings();
    return Object.keys(current).some(key => current[key] !== originalSettings[key]);
}

function updateSaveButton() {
    saveSettingsBtn.disabled = !hasSettingsChanged() || userName.value.trim() === "";
}

async function loadSettings() {
    try {
        const settings = await GetSettings();
        originalSettings = {
            user_name: settings.user_name || "",
            download_path: settings.download_path || "",
            is_permission_required_to_send_files: !!settings.is_permission_required_to_send_files
        };
        fillInputs(originalSettings);
        if (myDeviceName) myDeviceName.textContent = originalSettings.user_name || "This device";
    } catch (err) {
        console.error("Failed to load settings:", err);
    }
}

function discardAndClose() {
    fillInputs(originalSettings);
    closeModal(MODAL_ID);
}

loadSettings();

settingsButton.addEventListener("click", () => {
    showSection("general");
    openModal(MODAL_ID, {focus: ".modal-nav-item.active"});
});

closeSettingsButton.addEventListener("click", discardAndClose);
cancelSettingsBtn.addEventListener("click", discardAndClose);
onModalDismiss(MODAL_ID, discardAndClose);

userName.addEventListener("input", updateSaveButton);
permissionRequired.addEventListener("change", updateSaveButton);
userName.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !saveSettingsBtn.disabled) saveSettingsBtn.click();
});

saveSettingsBtn.addEventListener("click", async () => {
    const updated = currentSettings();
    saveSettingsBtn.disabled = true;
    try {
        await SaveSettings(updated);
        originalSettings = { ...updated };
        if (myDeviceName) myDeviceName.textContent = updated.user_name || "This device";
        showToast("Settings saved", "", "success");
        closeModal(MODAL_ID);
    } catch (err) {
        console.error("Failed to save settings", err);
        showToast("Couldn't save settings", "Please try again.", "error");
    } finally {
        updateSaveButton();
    }
});

folderPicker.addEventListener("click", async () => {
    try {
        const picked = await PickDownloadFolder();
        if (picked) {
            downloadPath.value = picked;
            downloadPath.title = picked;
            updateSaveButton();
        }
    } catch (e) {
        console.error("Error while setting download directory", e);
        showToast("Couldn't change the folder", "Please pick another location.", "error");
    }
});

const SECTION_COPY = {
    general: ["General", "Appearance, identity and how you receive files"],
    about: ["About", "Version, credits and support"],
};

function showSection(section) {
    document.querySelectorAll(".modal-nav-item").forEach(b => {
        const active = b.dataset.section === section;
        b.classList.toggle("active", active);
        if (active) b.setAttribute("aria-current", "page");
        else b.removeAttribute("aria-current");
    });
    document.querySelectorAll(".settings-section").forEach(s => s.classList.remove("active"));
    document.getElementById(`settings-${section}`).classList.add("active");

    const [title, subtitle] = SECTION_COPY[section];
    document.getElementById("settings-section-title").textContent = title;
    document.getElementById("settings-section-subtitle").textContent = subtitle;
    footer.hidden = section !== "general";
}

document.querySelectorAll(".modal-nav-item").forEach(btn => {
    btn.addEventListener("click", () => showSection(btn.dataset.section));
});

// Open About-page links in the system browser / mail client
document.getElementById("aboutPage").querySelectorAll("a").forEach(link => {
    link.addEventListener("click", (e) => {
        e.preventDefault();
        e.stopPropagation();
        const href = link.getAttribute("href");
        if (href) BrowserOpenURL(href);
    });
});
