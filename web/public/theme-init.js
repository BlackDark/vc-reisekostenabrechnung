(() => {
	var key = "mode-watcher-mode";
	var mode = localStorage.getItem(key);
	if (mode !== "light" && mode !== "dark" && mode !== "system") {
		mode = "dark";
		localStorage.setItem(key, mode);
	}
	var light =
		mode === "light" ||
		(mode === "system" &&
			window.matchMedia("(prefers-color-scheme: light)").matches);
	var root = document.documentElement;
	root.classList.toggle("dark", !light);
	root.style.colorScheme = light ? "light" : "dark";
	var meta = document.querySelector('meta[name="theme-color"]');
	if (meta) meta.setAttribute("content", light ? "#ffffff" : "#09090b");
})();
