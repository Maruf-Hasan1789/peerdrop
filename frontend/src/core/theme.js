// Appearance preference: "system" | "light" | "dark".
// Stored per device in localStorage; the inline script in index.html applies
// it before first paint so there is no flash of the wrong theme.

const STORAGE_KEY = "peerdrop.theme";
const PREFS = ["system", "light", "dark"];
const systemLight = window.matchMedia("(prefers-color-scheme: light)");

export function getThemePreference() {
    try {
        const value = localStorage.getItem(STORAGE_KEY);
        return PREFS.includes(value) ? value : "system";
    } catch {
        return "system";
    }
}

function resolve(pref) {
    if (pref === "system") return systemLight.matches ? "light" : "dark";
    return pref;
}

function applyTheme(pref) {
    document.documentElement.dataset.theme = resolve(pref);
}

function syncPicker(pref) {
    document.querySelectorAll(".theme-option").forEach(btn => {
        const active = btn.dataset.themeOption === pref;
        btn.classList.toggle("active", active);
        btn.setAttribute("aria-checked", String(active));
    });
}

export function setThemePreference(pref) {
    if (!PREFS.includes(pref)) return;
    try {
        localStorage.setItem(STORAGE_KEY, pref);
    } catch {
        // Storage unavailable: the choice still applies for this session.
    }
    applyTheme(pref);
    syncPicker(pref);
}

// Follow OS changes live while "System" is selected.
systemLight.addEventListener("change", () => {
    if (getThemePreference() === "system") applyTheme("system");
});

document.querySelectorAll(".theme-option").forEach(btn => {
    btn.addEventListener("click", () => setThemePreference(btn.dataset.themeOption));
});

applyTheme(getThemePreference());
syncPicker(getThemePreference());
