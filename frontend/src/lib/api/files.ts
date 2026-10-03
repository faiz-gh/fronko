import { ApiError, apiClient, apiUrl } from './client';
import { session } from '$lib/session.svelte';

export type FileKind = 'image' | 'pdf';

export interface LibraryFile {
	/** Public id: safe to put in card data and URLs. */
	id: string;
	kind: FileKind;
	content_type: string;
	size_bytes: number;
	name: string;
	title: string;
	created_at: string;
}

/** The subset a public card gets for the files it references. */
export interface PublicFile {
	id: string;
	kind: FileKind;
	name: string;
	size_bytes: number;
}

export interface FilePage {
	files: LibraryFile[];
	total: number;
	page: number;
	page_size: number;
}

export const MAX_IMAGE_BYTES = 5 * 1024 * 1024;
export const MAX_PDF_BYTES = 20 * 1024 * 1024;
export const IMAGE_TYPES = ['image/jpeg', 'image/png', 'image/webp'];
export const ACCEPT = { image: IMAGE_TYPES.join(','), pdf: 'application/pdf' } as const;

/** Where a file is served from; the server redirects to a short-lived signed URL. */
export function fileUrl(id: string): string {
	return apiUrl(`/api/files/${encodeURIComponent(id)}`);
}

export function listFiles(query: { kind?: FileKind; page?: number; pageSize?: number } = {}): Promise<FilePage> {
	const params = new URLSearchParams();
	if (query.kind) params.set('kind', query.kind);
	if (query.page) params.set('page', String(query.page));
	if (query.pageSize) params.set('page_size', String(query.pageSize));
	const qs = params.toString();
	return apiClient<FilePage>(`/api/me/files${qs ? `?${qs}` : ''}`);
}

export function renameFile(id: string, title: string): Promise<LibraryFile> {
	return apiClient<LibraryFile>(`/api/me/files/${encodeURIComponent(id)}`, {
		method: 'PATCH',
		body: JSON.stringify({ title })
	});
}

export function deleteFile(id: string): Promise<void> {
	return apiClient<void>(`/api/me/files/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

/** Client-side pre-check so users get instant feedback; the server checks again. */
export function checkUpload(file: File, kind?: FileKind): string | null {
	const isPdf = file.type === 'application/pdf' || /\.pdf$/i.test(file.name);
	const isImage = IMAGE_TYPES.includes(file.type);
	if (kind === 'image' && !isImage) return 'Choose a JPEG, PNG or WebP image.';
	if (kind === 'pdf' && !isPdf) return 'Choose a PDF file.';
	if (!isImage && !isPdf) return 'Only JPEG, PNG or WebP images and PDF files can be uploaded.';
	if (isImage && file.size > MAX_IMAGE_BYTES) return 'Images can be up to 5 MB.';
	if (isPdf && file.size > MAX_PDF_BYTES) return 'PDFs can be up to 20 MB.';
	return null;
}

/**
 * Uploads to the user's bucket via the API. Uses XMLHttpRequest because fetch
 * can't report upload progress.
 */
export function uploadFile(
	file: File,
	{ title = '', onProgress, signal }: { title?: string; onProgress?: (fraction: number) => void; signal?: AbortSignal } = {}
): Promise<LibraryFile> {
	return new Promise((resolve, reject) => {
		const xhr = new XMLHttpRequest();
		xhr.open('POST', apiUrl('/api/me/files'));
		// Send the session cookie when the API is on another origin (BACKEND_URL).
		xhr.withCredentials = true;
		xhr.responseType = 'json';
		xhr.upload.onprogress = (e) => {
			if (e.lengthComputable) onProgress?.(e.loaded / e.total);
		};
		xhr.onload = () => {
			if (xhr.status >= 200 && xhr.status < 300) {
				resolve(xhr.response as LibraryFile);
				return;
			}
			if (xhr.status === 401) session.expire();
			const message =
				(xhr.response && typeof xhr.response.error === 'string' && xhr.response.error) ||
				(xhr.status === 413 ? 'That file is too large.' : `Upload failed (${xhr.status})`);
			reject(new ApiError(message, xhr.status));
		};
		xhr.onerror = () => reject(new ApiError('Could not reach the server. Check your connection and try again.', 0));
		xhr.onabort = () => reject(new ApiError('Upload cancelled.', 0));
		signal?.addEventListener('abort', () => xhr.abort());

		const form = new FormData();
		if (title) form.set('title', title);
		form.set('file', file);
		xhr.send(form);
	});
}

export function formatBytes(bytes: number): string {
	if (bytes < 1024) return `${bytes} B`;
	if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
	return `${(bytes / (1024 * 1024)).toFixed(bytes < 10 * 1024 * 1024 ? 1 : 0)} MB`;
}
