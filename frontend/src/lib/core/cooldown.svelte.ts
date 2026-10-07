/** A countdown in whole seconds, e.g. for "Resend code in 42s". */
export class Cooldown {
	remaining = $state(0);
	#timer: ReturnType<typeof setInterval> | undefined;

	get active() {
		return this.remaining > 0;
	}

	start(seconds: number) {
		clearInterval(this.#timer);
		this.remaining = Math.ceil(seconds);
		this.#timer = setInterval(() => {
			this.remaining -= 1;
			if (this.remaining <= 0) this.stop();
		}, 1000);
	}

	stop() {
		clearInterval(this.#timer);
		this.remaining = 0;
	}
}

/** Matches the backend's resend cooldown. */
export const RESEND_COOLDOWN_SECONDS = 60;
