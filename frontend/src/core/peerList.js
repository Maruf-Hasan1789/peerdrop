import {EventsOn} from "../../wailsjs/runtime";
import {ListPeers} from "../../wailsjs/go/app/App";
import {cancelTransfer, clearReceiverSelection, ongoingTransfers, receiverSelect} from "../transfer/sender";

const peerListEl = document.getElementById("peer-list");
const activePeerCount = document.getElementById("active-peer-count");
const refreshBtn = document.getElementById("refresh-connected-peer-btn");

EventsOn("peer-connected", (peer) => {
    console.log("Peer Added from frontend")
    peersMap.set(peer.id, peer);
    updatePeerListUI();
    console.log("Peer " + peer)
    console.log(`[CONNECTED] ${peer.user_name} (${peer.id}) : ${peer.port}`);
});

EventsOn("peer-disconnected", async (peer) => {
    console.log(`[DISCONNECTED] ${peer.user_name} (${peer.id})`);

    // Cancel all transfers belonging to this peer
    for (const [transferId, transfer] of ongoingTransfers) {
        if (transfer.peerId === peer.id) {
            cancelTransfer(transferId, "peer-disconnected");
        }
    }

    peersMap.delete(peer.id);
    clearReceiverSelection();

    await sleep(1000);
    await fetchPeers();
});

// Local Map to store peers
const peersMap = new Map();

// Fetch peers from backend
export async function fetchPeers() {
    try {
        const peers = await ListPeers();
        console.log("Peer From Backend " + peers)
        peersMap.clear();
        peers.forEach(peer => peersMap.set(peer.id, peer));
        updatePeerListUI();
    } catch (err) {
        console.error("Failed to fetch peers:", err);
    }
}

// Update the DOM

export function updatePeerListUI() {
    console.log("Updating Peer List...");

    // Clear the list and the dropdown
    peerListEl.innerHTML = '';

    peersMap.forEach(peer => {
        // --- Right sidebar list ---
        const li = document.createElement("li");
        //li.textContent = `${peer.name} (${peer.id}) - Port: ${peer.port}`;
        li.classList.add("peer-item");
        li.dataset.peerId = peer.id;

        console.log("Peer " + peer.id, peer.name, peer.user_name)

        li.innerHTML = `<strong>${peer.user_name}</strong><br>
                        <small>Port ${peer.port}</small>`

        // Click to highlight in sidebar
        li.addEventListener("click", () => {
            document.querySelectorAll("#peer-list li").forEach(el => el.classList.remove("selected"));
            li.classList.add("selected");

            renderSelectedPeer(peer)
        });

        peerListEl.appendChild(li);
    });

    activePeerCount.textContent = peersMap.size.toString();
}

function renderSelectedPeer(peer) {
    receiverSelect.value = peer.id;

    document.getElementById("selected-peer-name").textContent = peer.user_name
    document.getElementById("selected-peer-status").textContent = `Port ${peer.port} ${peer.id}`;

    const card = document.getElementById("selected-peer-card")
    card.classList.add("active");
    document.getElementById("clear-selection").style.display = "flex";
}

function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms))
}

document.addEventListener("DOMContentLoaded", () => {
    refreshBtn?.addEventListener("click", fetchPeers);
});

updatePeerListUI();