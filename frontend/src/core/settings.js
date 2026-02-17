import { GetSettings, PickDownloadFolder, ReceiveFilePermission, SaveSettings } from "../../wailsjs/go/app/App";
import {BrowserOpenURL, EventsOn} from "../../wailsjs/runtime";

const settingsModal = document.getElementById("settings-modal");
const settingsButton = document.getElementById("settings-toggle");
const closeSettingsButton = document.getElementById("close-settings");
const saveSettingsBtn = document.getElementById("saveSettingsButton");
const userName = document.getElementById("userName");
const downloadPath = document.getElementById("downloadPath");
const folderPicker = document.getElementById("folderPicker");
const cancelSettingsBtn = document.getElementById("settingsCancelButton");
const permissionRequired = document.getElementById('permissionToggle');

let originalSettings = {};

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


function hasSettingsChanged() {
    const currentSettings = {
        user_name: userName.value,
        download_path: downloadPath.value,
        is_permission_required_to_send_files: permissionRequired.checked
    };

    return Object.keys(currentSettings).some(key => currentSettings[key] !== originalSettings[key]);
}

const navButtons = document.querySelectorAll(".modal-nav-item");
const footer = document.querySelector(".modal-footer"); // footer container

navButtons.forEach(btn => {
    btn.addEventListener("click", () => {
        const section = btn.dataset.section; // "general" or "about"

        // Remove "active" from all buttons
        navButtons.forEach(b => b.classList.remove("active"));

        // Remove "active" from all sections
        document.querySelectorAll(".settings-section").forEach(s => s.classList.remove("active"));

        // Activate clicked button
        btn.classList.add("active");

        // Show corresponding section
        document.getElementById(`settings-${section}`).classList.add("active");

        // Update header title and subtitle
        const titleEl = document.getElementById("settings-section-title");
        const subtitleEl = document.getElementById("settings-section-subtitle");

        if (section === "general") {
            titleEl.textContent = "General";
            subtitleEl.textContent = "Manage basic application preferences";
            footer.style.display = "flex"; // show footer
        } else if (section === "about") {
            titleEl.textContent = "About";
            subtitleEl.textContent = "";
            footer.style.display = "none"; // hide footer
        }
    });
});

// Select the About page container
const aboutPage = document.getElementById("aboutPage");

// Add a click handler for all links inside the About page
aboutPage.querySelectorAll("a").forEach(link => {
    link.addEventListener("click", (e) => {
        e.stopPropagation(); // prevent modal or default interference
        e.preventDefault();
        const href = link.getAttribute("href");

        if (href) {
            BrowserOpenURL(href)
        }
    });
});

