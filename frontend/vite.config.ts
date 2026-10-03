import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
	plugins: [
		sveltekit({
			adapter: adapter({
				fallback: 'index.html'
			}),
			preprocess: vitePreprocess()
		})
	],
	server: {
		proxy: {
			'/api': 'http://localhost:8080',
			'/auth': 'http://localhost:8080'
		}
	}
});
