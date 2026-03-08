
export function showToast(title, message) {
    const toast = document.createElement("div");
    toast.className = "toast";

    toast.innerHTML = `
        <strong>${title}</strong>
        <div>${message}</div>
    `;

    document.body.appendChild(toast);

    setTimeout(() => toast.classList.add("show"), 10);

    setTimeout(() => {
        toast.classList.remove("show");
        setTimeout(() => toast.remove(), 300);
    }, 5000);
}