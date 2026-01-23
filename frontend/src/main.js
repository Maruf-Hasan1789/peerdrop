console.log("🔥 frontend main.js loaded");
import './style.css'

import  {ListPeers, SendFileToPeer, PickFile}  from '../wailsjs/go/app/App';
import {EventsOn} from "../wailsjs/runtime";

console.log("🔥 imports resolved");

const peerListEl = document.getElementById("peer-list");
const refreshBtn = document.getElementById("refresh-btn");
const sendFileButton = document.getElementById("send-btn");
const pickFile = document.getElementById("pick-file");
const fileName = document.getElementById("file-name");
const clearSelection = document.getElementById("clear-selection");
let filePath =  "";
const receiverSelect = document.getElementById("receiver-select");
// Local Map to store peers
const peersMap = new Map();

// Fetch peers from backend
async function fetchPeers() {
    try {
        const peers = await ListPeers();
        peersMap.clear();
        peers.forEach(peer => peersMap.set(peer.id, peer));
        updatePeerListUI();
    } catch (err) {
        console.error("Failed to fetch peers:", err);
    }
}

// Update the DOM

function updatePeerListUI() {
    console.log("Updating Peer List...");

    // Clear the list and the dropdown
    peerListEl.innerHTML = '';

    peersMap.forEach(peer => {
        // --- Right sidebar list ---
        const li = document.createElement("li");
        //li.textContent = `${peer.name} (${peer.id}) - Port: ${peer.port}`;
        li.classList.add("peer-item");
        li.dataset.peerId = peer.id;
        li.innerHTML = `<strong>${peer.name}</strong><br>
                        <small>Port ${peer.port}</small>`

        // Click to highlight in sidebar
        li.addEventListener("click", () => {
            document.querySelectorAll("#peer-list li").forEach(el => el.classList.remove("selected"));
            li.classList.add("selected");

           renderSelectedPeer(peer)
        });

        peerListEl.appendChild(li);
    });
}


document.addEventListener("DOMContentLoaded", () => {
    refreshBtn?.addEventListener("click", fetchPeers);
});



window.addEventListener('dragover', (e) => e.preventDefault());
window.addEventListener('drop', (e) => e.preventDefault());
EventsOn("peer-connected", (peer) => {
    console.log("Peer Added from frontend")
    peersMap.set(peer.id, peer);
    updatePeerListUI()
    console.log(`[CONNECTED] ${peer.name} (${peer.id}) : ${peer.port}`);
});

EventsOn("peer-disconnected", (peer) => {
    peersMap.delete(peer.id);
    fetchPeers().then(r => {
        console.log(r)
    });

    console.log(`[DISCONNECTED] ${peer.name} (${peer.id})`);
});

EventsOn("file-received", (data) => {
    console.log(`[FILE] ${data.peer} -> ${data.name}`);
});

EventsOn("randomEvent", (data) => {
   console.log("random event from frontend " + data)
});


EventsOn("wails:file-drop", (x,y,paths) =>{
    console.log("Dropped files at: ",x,y);
    console.log("File paths: ", paths)

    if(paths && paths.length > 0) {
        console.log("File received via Drag & Drop");
        handleFileSelection(paths[0]);
    }
})

sendFileButton.addEventListener("click", async (event) => {
    event.preventDefault();
    console.log("Send Button is selected");
    try {
        // Open native file dialog via Go
        console.log("filePath", filePath);
        if(filePath === "") {
            console.error("File Path is not provided")
            return
        }

        console.log("receiver ", receiverSelect.value, "filePath");
        // Send file to selected peer
        await SendFileToPeer(receiverSelect.value, filePath);
        sendFileButton.disabled = true;
        console.log("File sent:", filePath);
    } catch (err) {
        console.error("Error while sending file:", err);
    }
});

pickFile.addEventListener("click", async () => {
    console.log("Picking File")
    filePath = await PickFile();
    handleFileSelection(filePath);
});

function getFileName(path) {
    return path.split(/[/\\]/).pop();
}

function handleFileSelection(path) {
    if(!path || path === "") {
        fileName.textContent = "No file selected";
        sendFileButton.disabled = true;
        fileName.textContent = "";
        return;
    }

    filePath = path;
    fileName.textContent = getFileName(path)
    sendFileButton.disabled = false;
    console.log("File Ready to send", filePath)
}

console.log("Hello");
updatePeerListUI();


function renderSelectedPeer(peer) {
    receiverSelect.value = peer.id;

    document.getElementById("selected-peer-name").textContent = peer.name
    document.getElementById("selected-peer-status").textContent = `Port ${peer.port} ${peer.id}`;

   // if (peer.avatar) {
       // document.getElementById("selected-peer-image").src = peer.avatar;
  //  }

    const card = document.getElementById("selected-peer-card")
    card.classList.add("active");
    document.getElementById("clear-selection").style.display = "flex";
}

clearSelection.addEventListener(("click"), () => {
    document.getElementById("receiver-select").value = "";

    document.getElementById("selected-peer-name").textContent = "No Peer Selected";
    document.getElementById("selected-peer-status").textContent = "Selected a Peer from the sidebar";

    document.querySelectorAll("#peer-list li")
        .forEach(el => el.classList.remove("selected"));
});