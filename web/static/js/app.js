// Dispatch v2 - JS for htmx enhancements

// Mobile sidebar toggle
function toggleSidebar() {
    var sidebar = document.getElementById("sidebar");
    var backdrop = document.getElementById("sidebar-backdrop");
    if (!sidebar) return;
    sidebar.classList.toggle("open");
    if (backdrop) backdrop.classList.toggle("hidden");
}

// Close sidebar on navigation (mobile)
document.body.addEventListener("htmx:beforeRequest", function () {
    var sidebar = document.getElementById("sidebar");
    var backdrop = document.getElementById("sidebar-backdrop");
    if (sidebar && window.innerWidth < 768) {
        sidebar.classList.remove("open");
        if (backdrop) backdrop.classList.add("hidden");
    }
});

// Toast notifications triggered by HX-Trigger response header
document.body.addEventListener("showToast", function (e) {
    const { message, type } = e.detail;
    showToast(message, type || "info");
});

function showToast(message, type) {
    const container = document.getElementById("toast-container");
    if (!container) return;

    const toast = document.createElement("div");
    toast.className = `toast toast-${type}`;
    toast.textContent = message;
    container.appendChild(toast);

    setTimeout(function () {
        toast.style.opacity = "0";
        toast.style.transform = "translateX(100%)";
        toast.addEventListener("transitionend", function () {
            toast.remove();
        });
    }, 3000);
}

// Handle 401 responses from htmx requests (session expired)
document.body.addEventListener("htmx:responseError", function (e) {
    if (e.detail.xhr.status === 401) {
        window.location.href = "/login";
    }
});

// Command palette (Cmd+K / Ctrl+K)
document.addEventListener("keydown", function (e) {
    if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        const dialog = document.getElementById("command-palette");
        if (!dialog) return;
        if (dialog.open) {
            dialog.close();
        } else {
            dialog.showModal();
            const input = dialog.querySelector("input");
            if (input) {
                input.value = "";
                input.focus();
            }
        }
    }
});

// Close command palette on Escape or backdrop click
document.addEventListener("click", function (e) {
    const dialog = document.getElementById("command-palette");
    if (dialog && dialog.open && e.target === dialog) {
        dialog.close();
    }
});

// Dynamic row management for advanced step forms
function addRow(container) {
    var tmpl = container.querySelector("[data-row-template]");
    if (!tmpl) return;
    var countInput = container.querySelector("[data-count]");
    var count = parseInt(countInput.value, 10);
    var html = tmpl.innerHTML.replace(/__IDX__/g, count);
    var wrapper = document.createElement("div");
    wrapper.innerHTML = html.trim();
    var row = wrapper.firstElementChild;
    container.querySelector("[data-rows]").appendChild(row);
    countInput.value = count + 1;
}

function removeRow(btn) {
    var row = btn.closest("[data-row]");
    var container = row.closest("[data-dynamic]");
    row.remove();
    reindexRows(container);
}

function reindexRows(container) {
    var rows = container.querySelectorAll("[data-row]");
    rows.forEach(function (row, i) {
        row.querySelectorAll("input,select,textarea").forEach(function (input) {
            input.name = input.name.replace(/__\d+__/, "__" + i + "__");
        });
    });
    var countInput = container.querySelector("[data-count]");
    if (countInput) countInput.value = rows.length;
}
