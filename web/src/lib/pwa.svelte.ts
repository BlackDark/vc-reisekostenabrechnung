type BeforeInstallPromptEvent = Event & {
	prompt: () => Promise<void>;
	userChoice: Promise<{ outcome: "accepted" | "dismissed" }>;
};

/** iOS and iPadOS never fire beforeinstallprompt, they install from the share sheet. */
export function isIOS(userAgent: string, maxTouchPoints: number): boolean {
	return (
		/iPad|iPhone|iPod/.test(userAgent) ||
		(userAgent.includes("Macintosh") && maxTouchPoints > 1)
	);
}

/** SPEC §13: install button plus the iOS hint, so every platform has a way in. */
class Pwa {
	/** True while the browser still offers its native install prompt. */
	canInstall = $state(false);
	/** True once the app runs standalone, then there is nothing left to install. */
	installed = $state(false);
	/** True on iOS/iPadOS, which need the share-sheet hint instead of a prompt. */
	ios = $state(false);
	#pending: BeforeInstallPromptEvent | null = null;

	init() {
		if (typeof window === "undefined") return;
		const nav = navigator as Navigator & { standalone?: boolean };
		this.ios = isIOS(nav.userAgent, nav.maxTouchPoints ?? 0);
		this.installed =
			nav.standalone === true ||
			window.matchMedia("(display-mode: standalone)").matches;
		addEventListener("beforeinstallprompt", (event) => {
			event.preventDefault();
			this.#pending = event as BeforeInstallPromptEvent;
			this.canInstall = true;
		});
		addEventListener("appinstalled", () => {
			this.#pending = null;
			this.canInstall = false;
			this.installed = true;
		});
	}

	/** Returns whether the user accepted; the prompt is consumed either way. */
	async promptInstall(): Promise<boolean> {
		const pending = this.#pending;
		if (!pending || this.installed) return false;
		this.#pending = null;
		this.canInstall = false;
		await pending.prompt();
		const { outcome } = await pending.userChoice;
		return outcome === "accepted";
	}
}

export const pwa = new Pwa();
