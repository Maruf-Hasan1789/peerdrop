// Shared modal behaviour: open/close, Esc, click-outside and focus handling.
// Overlays marked `data-dismissible` close on Esc or a backdrop click;
// a module can intercept that with onModalDismiss (e.g. to revert edits).

const dismissHandlers = new Map();
const lastFocus = new Map();

function overlay(id) {
    return typeof id === "string" ? document.getElementById(id) : id;
}

export function isModalOpen(id) {
    const el = overlay(id);
    return !!el && el.classList.contains("is-open");
}

export function openModal(id, {focus} = {}) {
    const el = overlay(id);
    if (!el || isModalOpen(el)) return;
    lastFocus.set(el.id, document.activeElement);
    el.style.display = "flex";
    el.classList.add("is-open");
    const target = focus
        ? el.querySelector(focus)
        : el.querySelector("[autofocus]") || el.querySelector(".modal");
    if (target) {
        if (target.classList.contains("modal")) target.tabIndex = -1;
        requestAnimationFrame(() => target.focus({preventScroll: true}));
    }
}

export function closeModal(id) {
    const el = overlay(id);
    if (!el || !isModalOpen(el)) return;
    el.classList.remove("is-open");
    el.style.display = "none";
    const prev = lastFocus.get(el.id);
    if (prev && document.contains(prev)) prev.focus({preventScroll: true});
}

/** Register what happens when the user dismisses (Esc / backdrop) a modal. */
export function onModalDismiss(id, handler) {
    dismissHandlers.set(id, handler);
}

function requestDismiss(el) {
    if (!el.hasAttribute("data-dismissible")) return;
    const handler = dismissHandlers.get(el.id);
    if (handler) handler();
    else closeModal(el);
}

document.addEventListener("keydown", (e) => {
    if (e.key !== "Escape") return;
    const open = [...document.querySelectorAll(".modal-overlay.is-open")];
    const top = open[open.length - 1];
    if (top) {
        e.preventDefault();
        requestDismiss(top);
    }
});

document.querySelectorAll(".modal-overlay").forEach(el => {
    // Only a press that starts and ends on the backdrop counts as "outside".
    let downOnBackdrop = false;
    el.addEventListener("mousedown", (e) => { downOnBackdrop = e.target === el; });
    el.addEventListener("click", (e) => {
        if (downOnBackdrop && e.target === el) requestDismiss(el);
        downOnBackdrop = false;
    });
});
