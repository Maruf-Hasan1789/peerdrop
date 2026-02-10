import {GetSettings, PickDownloadFolder, ReceiveFilePermission, SaveSettings} from "../../wailsjs/go/app/App";
import {EventsOn} from "../../wailsjs/runtime";

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
        originalSettings = {...settings}

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
        originalSettings = {...updatedSettings}
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
        li.textContent = `File Name: ${file.file_name} Size: ${decimalNumberFormatter.format((file.file_size) / (1024 * 1024))} MB`;
        fileList.appendChild(li);
    });

    modal.style.display = "flex";

    document.getElementById("allow-btn").onclick = () => {
        ReceiveFilePermission(sender.id, sender.files[0].name, sender.files[0].id, true).then(r => console.log("Permission granted"));
        modal.style.display = "none";
    };

    document.getElementById("deny-btn").onclick = () => {
        ReceiveFilePermission(sender.id, sender.files[0].name, sender.files[0].id, false).then(r => console.log("Permission denied"));
        modal.style.display = "none";
    };
}

const decimalNumberFormatter = new Intl.NumberFormat('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
});
