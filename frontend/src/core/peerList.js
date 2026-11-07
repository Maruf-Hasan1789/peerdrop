import {EventsOn} from "../../wailsjs/runtime";
import {ListPeers} from "../../wailsjs/go/app/App";
import {cancelTransfer, clearReceiverSelection, ongoingTransfers, receiverSelect} from "../transfer/sender";
import {escapeHtml, hueOf, initialsOf} from "../utils/utils";

const peerListEl = document.getElementById("peer-list");
const activePeerCount = document.getElementById("active-peer-count");
const peerListEmpty = document.getElementById("peer-list-empty");
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

function avatarStyle(peer) {
    const hue = hueOf(peer.id || peer.user_name);
    return `--avatar-hue: ${hue}`;
}

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
        const li = document.createElement("li");
        li.classList.add("peer-item");
        li.dataset.peerId = peer.id;

        li.tabIndex = 0;
        li.setAttribute("role", "button");
        if (receiverSelect.value === peer.id) li.classList.add("selected");

        li.innerHTML = `
            <div class="peer-avatar" style="${avatarStyle(peer)}" aria-hidden="true">
                ${escapeHtml(initialsOf(peer.user_name || "Peer"))}
                <span class="presence-dot"></span>
            </div>
            <div class="peer-info">
                <span class="peer-name">${escapeHtml(peer.user_name || 'Peer')}</span>
                <span class="peer-meta">Online · Port ${escapeHtml(String(peer.port))}</span>
            </div>
            <span class="peer-check material-symbols-outlined" aria-hidden="true">check_circle</span>
        `;
        li.addEventListener("keydown", (e) => {
            if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                li.click();
            }
        });

        li.addEventListener("click", () => {
            document.querySelectorAll("#peer-list li").forEach(el => el.classList.remove("selected"));
            li.classList.add("selected");
            renderSelectedPeer(peer);
        });

        peerListEl.appendChild(li);
    });

    if (activePeerCount) activePeerCount.textContent = peersMap.size.toString();
    if (peerListEmpty) peerListEmpty.hidden = peersMap.size > 0;
}

function renderSelectedPeer(peer) {
    receiverSelect.value = peer.id;

    document.getElementById("selected-peer-name").textContent = peer.user_name || "Peer";
    document.getElementById("selected-peer-status").textContent = `Port ${peer.port}`;

    const avatar = document.getElementById("selected-peer-avatar");
    avatar.setAttribute("style", avatarStyle(peer));
    avatar.textContent = initialsOf(peer.user_name || "Peer");

    const card = document.getElementById("selected-peer-card")
    card.classList.remove("empty-state");
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