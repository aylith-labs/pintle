const trigger = document.querySelector("#theme-trigger"),
	list = document.querySelector("#theme-list");
const options = [...list.querySelectorAll("[role=option]")],
	allowed = ["system", "light", "dark"];
function selected(value) {
	if (!allowed.includes(value)) value = "system";
	document.documentElement.dataset.theme =
		value === "system"
			? matchMedia("(prefers-color-scheme:dark)").matches
				? "dark"
				: "light"
			: value;
	trigger.textContent = `Theme · ${value[0].toUpperCase() + value.slice(1)}`;
	options.forEach((b) => {
		b.setAttribute("aria-selected", String(b.dataset.theme === value));
	});
	return value;
}
let value = "system";
try {
	value = localStorage.getItem("pintle-theme") || "system";
} catch {}
value = selected(value);
function close(focus = false) {
	list.hidden = true;
	trigger.setAttribute("aria-expanded", "false");
	if (focus) trigger.focus();
}
trigger.addEventListener("click", () => {
	if (!list.hidden) return close();
	list.hidden = false;
	trigger.setAttribute("aria-expanded", "true");
	options.find((b) => b.dataset.theme === value).focus();
});
options.forEach((b, i) => {
	b.addEventListener("click", () => {
		value = selected(b.dataset.theme);
		try {
			localStorage.setItem("pintle-theme", value);
		} catch {}
		close(true);
	});
	b.addEventListener("keydown", (e) => {
		if (["ArrowDown", "ArrowUp", "Home", "End"].includes(e.key)) {
			e.preventDefault();
			options[
				e.key === "Home" ? 0 : e.key === "End" ? 2 : (i + (e.key === "ArrowDown" ? 1 : 2)) % 3
			].focus();
		}
		if (e.key === "Escape") {
			e.preventDefault();
			close(true);
		}
	});
});
trigger.addEventListener("keydown", (e) => {
	if (e.key === "ArrowDown") {
		e.preventDefault();
		trigger.click();
	}
	if (e.key === "Escape") close(true);
});
document.addEventListener("click", (e) => {
	if (!e.target.closest(".theme")) close();
});
matchMedia("(prefers-color-scheme:dark)").addEventListener("change", () => {
	if (value === "system") selected(value);
});
document.querySelector("#copy").addEventListener("click", async () => {
	const status = document.querySelector("#copy-status");
	try {
		await navigator.clipboard.writeText(document.querySelector("#install-command").textContent);
		status.textContent = "Copied";
	} catch {
		status.textContent = "Select and copy the commands above";
	}
});
