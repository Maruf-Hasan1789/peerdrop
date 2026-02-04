import "./core/peerList";
import "./core/settings";
import "./core/state";
import './style.css';
import "./transfer/receiver";
import "./transfer/sender";
console.log("🔥 frontend main.js loaded");


window.addEventListener('dragover', (e) => e.preventDefault());
window.addEventListener('drop', (e) => e.preventDefault());
