import "./core/peerList";
import "./core/settings";
import "./core/state";
import './style.css';
import "./transfer/receiver";
import "./transfer/sender";
import * as runtime from "../wailsjs/runtime";
import {EventsOn} from "../wailsjs/runtime";
import {handleFileSelection} from "./transfer/sender";
console.log("🔥 frontend main.js loaded");


runtime.OnFileDrop((x, y, paths) => {
    console.log("Dropped file paths:", paths)
}, true)



window.addEventListener('dragover', (e) => e.preventDefault());
window.addEventListener('drop', (e) => e.preventDefault());

document.addEventListener("DOMContentLoaded", () => {
    EventsOn("wails:file-drop", (x, y, paths) => {
        console.log("DROP EVENT FIRED");
        console.log("Dropped paths:", paths);
        handleFileSelection(paths);
    });
});