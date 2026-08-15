import { api, type User } from './api';

class AuthStore {
	user = $state<User | null>(null);
	loaded = $state(false);

	async load() {
		try {
			this.user = await api.me();
		} catch {
			this.user = null;
		} finally {
			this.loaded = true;
		}
	}

	async login(email: string, password: string) {
		this.user = await api.login(email, password);
		this.loaded = true;
	}

	async signup(email: string, password: string) {
		this.user = await api.signup(email, password);
		this.loaded = true;
	}

	async logout() {
		try {
			await api.logout();
		} finally {
			this.user = null;
		}
	}
}

export const auth = new AuthStore();
