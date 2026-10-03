import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			adapter: adapter({
				fallback: 'index.html'
			}),
			preprocess: vitePreprocess(),
			// SvelteKit 3 no longer provides $lib by default; keep it until imports move to #lib.
			alias: {
				'$lib': 'src/lib'
			}
		})
	],
	server: {
		// Keep the browser's Host header so the backend's same-origin check matches Origin.
		proxy: {
			'/api': { target: 'http://localhost:8080', changeOrigin: false },
			'/auth': { target: 'http://localhost:8080', changeOrigin: false }
		}
	}
});
