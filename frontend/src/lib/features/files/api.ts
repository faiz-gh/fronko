import { ApiError, apiClient, apiUrl } from '$lib/core/api';
import type { TeamRef } from '$lib/features/teams/api';
import type { CropRatio, ImageFormat } from '$lib/core/image';
import { makeThumb } from './thumbnails';
import { session } from '$lib/core/session.svelte';

export type FileKind = 'image' | 'pdf';

/**
 * A user's own files, the organisation's private files, the area everyone in
 * it can see, or a team's files.
 */
export type FileArea = 'personal' | 'org' | 'shared' | 'team';

/** What a file is for. */
export type FilePurpose = 'logo' | 'banner' | 'avatar' | 'cover' | 'gallery' | 'brochure' | 'other';

export interface LibraryFile {
	/** Public id: safe to put in card data and URLs. */
	id: string;
	area: FileArea;
	/** Whose personal file this is, or who uploaded an organisation, shared or team file. */
	owner?: { id: number; username: string };
	/** Username of a deleted user whose personal file this was. */
	former_owner?: string;
	/** The team whose area the file is in. */
	team?: TeamRef;
	kind: FileKind;
	purpose: FilePurpose;
	content_type: string;
	size_bytes: number;
	name: string;
	title: string;
	/** Images: their size in pixels, when known. */
	width: number | null;
	height: number | null;
	/** PDFs: the page count, when known. */
	pages: number | null;
	/** There's a small preview at fileUrl(id, { thumb: true }). */
	has_thumb: boolean;
	/** Cards using it, plus the organisation's logo and signature banner. */
	use_count: number;
	created_at: string;
	updated_at: string;
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

/** How an image for a purpose is framed before it's saved. */
export interface CropSpec {
	title: string;
	aspect: number;
	shape?: 'rect' | 'round';
	width: number;
	height: number;
	format?: ImageFormat;
	/** Shapes to choose from; the first is the default. */
	ratios?: CropRatio[];
}

export const BANNER_RATIOS: CropRatio[] = [
	{ label: '4:1', aspect: 4, width: 1200, height: 300 },
	{ label: '3:1', aspect: 3, width: 1200, height: 400 },
	{ label: '2:1', aspect: 2, width: 1200, height: 600 }
];

export interface PurposeInfo {
	label: string;
	/** For filters: "Logos", "Brochures". */
	plural: string;
	kind: FileKind;
	hint: string;
	/** Images for this purpose are cropped to this shape. */
	crop?: CropSpec;
}

/** Every purpose, in the order the library shows them. */
export const PURPOSES: Record<FilePurpose, PurposeInfo> = {
	logo: {
		label: 'Logo',
		plural: 'Logos',
		kind: 'image',
		hint: 'Shown on cards and email signatures',
		crop: { title: 'Crop logo', aspect: 1, width: 512, height: 512, format: 'png' }
	},
	banner: {
		label: 'Banner',
		plural: 'Banners',
		kind: 'image',
		hint: 'Wide images under email signatures',
		crop: { title: 'Crop banner', aspect: 4, width: 1200, height: 300, format: 'jpeg', ratios: BANNER_RATIOS }
	},
	avatar: {
		label: 'Profile photo',
		plural: 'Profile photos',
		kind: 'image',
		hint: 'The round photo at the top of a card',
		crop: { title: 'Crop photo', aspect: 1, shape: 'round', width: 512, height: 512 }
	},
	cover: {
		label: 'Cover',
		plural: 'Covers',
		kind: 'image',
		hint: 'The wide image behind a card’s header',
		crop: { title: 'Crop cover image', aspect: 3, width: 1500, height: 500 }
	},
	gallery: { label: 'Gallery', plural: 'Gallery', kind: 'image', hint: 'Photos in a card’s gallery' },
	brochure: { label: 'Brochure', plural: 'Brochures', kind: 'pdf', hint: 'PDFs visitors can open from a card' },
	other: { label: 'Other', plural: 'Other', kind: 'image', hint: 'Anything else' }
};

export const PURPOSE_ORDER = Object.keys(PURPOSES) as FilePurpose[];

/** The purposes a kind of file can have: PDFs are brochures (or other), images anything else. */
export function purposesFor(kind: FileKind): FilePurpose[] {
	return PURPOSE_ORDER.filter((p) => p === 'other' || PURPOSES[p].kind === kind);
}

/** What a new upload is for when nobody says. */
export function defaultPurpose(kind: FileKind, preferred?: FilePurpose | null): FilePurpose {
	if (preferred && purposesFor(kind).includes(preferred)) return preferred;
	return kind === 'pdf' ? 'brochure' : 'other';
}

export function kindOf(file: File): FileKind {
	return file.type === 'application/pdf' || /\.pdf$/i.test(file.name) ? 'pdf' : 'image';
}

/** A file's display name. */
export function fileTitle(f: Pick<LibraryFile, 'title' | 'name'>): string {
	return f.title || f.name;
}

/**
 * Where a file is served from; the server redirects to a short-lived signed
 * URL. `thumb` asks for the small preview (the file itself if there is none).
 */
export function fileUrl(id: string, { thumb = false }: { thumb?: boolean } = {}): string {
	return apiUrl(`/api/files/${encodeURIComponent(id)}${thumb ? '?size=thumb' : ''}`);
}

export type FileSort = 'newest' | 'oldest' | 'name' | 'size';

export const SORT_LABEL: Record<FileSort, string> = {
	newest: 'Newest first',
	oldest: 'Oldest first',
	name: 'Name',
	size: 'Largest first'
};

export interface FileQuery {
	kind?: FileKind;
	/** "granted": files granted to the signed-in user or their teams (or, for admins, to userId). */
	area?: FileArea | 'granted';
	/** One team's files. */
	teamId?: number;
	/** Admins only: one user's files. */
	userId?: number;
	purposes?: FilePurpose[];
	/** Searches titles and file names. */
	q?: string;
	sort?: FileSort;
	page?: number;
	pageSize?: number;
}

function queryString(query: FileQuery): string {
	const params = new URLSearchParams();
	if (query.kind) params.set('kind', query.kind);
	if (query.area) params.set('area', query.area);
	if (query.teamId) params.set('team_id', String(query.teamId));
	if (query.userId) params.set('user_id', String(query.userId));
	if (query.purposes?.length) params.set('purpose', query.purposes.join(','));
	if (query.q?.trim()) params.set('q', query.q.trim());
	if (query.sort && query.sort !== 'newest') params.set('sort', query.sort);
	if (query.page) params.set('page', String(query.page));
	if (query.pageSize) params.set('page_size', String(query.pageSize));
	const qs = params.toString();
	return qs ? `?${qs}` : '';
}

/** Files the signed-in user can see: the whole organisation's for admins. */
export function listFiles(query: FileQuery = {}): Promise<FilePage> {
	return apiClient<FilePage>(`/api/me/files${queryString(query)}`);
}

/** How many files match the query for each purpose (the query's purposes are ignored). */
export function fileCounts(
	query: FileQuery = {}
): Promise<{ counts: Partial<Record<FilePurpose, number>>; total: number }> {
	return apiClient(
		`/api/me/files/counts${queryString({ ...query, purposes: undefined, page: undefined, pageSize: undefined, sort: undefined })}`
	);
}

export interface FilePatch {
	title?: string;
	purpose?: FilePurpose;
	/** Moves the file; team_id is needed for the team area. */
	area?: FileArea;
	team_id?: number;
}

export function updateFile(id: string, patch: FilePatch): Promise<LibraryFile> {
	return apiClient<LibraryFile>(`/api/me/files/${encodeURIComponent(id)}`, {
		method: 'PATCH',
		body: JSON.stringify(patch)
	});
}

export function deleteFile(id: string): Promise<void> {
	return apiClient<void>(`/api/me/files/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export interface BulkResult {
	done: string[];
	failed: { id: string; error: string }[];
}

/** Deletes or updates up to 100 files; each succeeds or fails on its own. */
export function bulkFiles(ids: string[], action: 'delete'): Promise<BulkResult>;
export function bulkFiles(ids: string[], action: 'update', patch: FilePatch): Promise<BulkResult>;
export function bulkFiles(ids: string[], action: 'delete' | 'update', patch?: FilePatch): Promise<BulkResult> {
	return apiClient<BulkResult>('/api/me/files/bulk', { method: 'POST', body: JSON.stringify({ ids, action, patch }) });
}

export type FileSlot = 'avatar' | 'cover' | 'document' | 'gallery';

export const SLOT_LABEL: Record<FileSlot, string> = {
	avatar: 'Profile photo',
	cover: 'Cover',
	document: 'Brochure',
	gallery: 'Gallery'
};

export interface FileUsage {
	cards: { profile_id: number; slug: string; name: string; slot: FileSlot }[];
	/** Cards using the file that the signed-in user can't see. */
	hidden_cards: number;
	org_logo: boolean;
	signature_banner: boolean;
}

export function fileUsage(id: string): Promise<FileUsage> {
	return apiClient<FileUsage>(`/api/me/files/${encodeURIComponent(id)}/usage`);
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

export interface UploadOptions {
	title?: string;
	area?: FileArea;
	/** Needed for the team area. */
	teamId?: number;
	purpose?: FilePurpose;
	onProgress?: (fraction: number) => void;
	signal?: AbortSignal;
}

/**
 * Uploads to the organisation's bucket via the API, with a small preview made
 * in the browser. Members' uploads go to their personal files; team leads can
 * pick their teams; admins the organisation's files (the default), the shared
 * area or a team. Uses XMLHttpRequest because fetch can't report upload progress.
 */
export async function uploadFile(file: File, options: UploadOptions = {}): Promise<LibraryFile> {
	const { title = '', area, teamId, purpose, onProgress, signal } = options;
	const preview = await makeThumb(file);
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
		if (area) form.set('area', area);
		if (teamId) form.set('team_id', String(teamId));
		if (purpose) form.set('purpose', purpose);
		if (preview?.width) form.set('width', String(preview.width));
		if (preview?.height) form.set('height', String(preview.height));
		if (preview?.pages) form.set('pages', String(preview.pages));
		// The file goes before the preview, so a dropped preview never holds the file up.
		form.set('file', file);
		if (preview?.thumb) form.set('thumb', preview.thumb, 'preview.webp');
		xhr.send(form);
	});
}

export function formatBytes(bytes: number): string {
	if (bytes < 1024) return `${bytes} B`;
	if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
	if (bytes < 1024 ** 3) return `${(bytes / (1024 * 1024)).toFixed(bytes < 10 * 1024 * 1024 ? 1 : 0)} MB`;
	return `${(bytes / 1024 ** 3).toFixed(bytes < 10 * 1024 ** 3 ? 1 : 0)} GB`;
}

/** "1200 × 300" or "4 pages", when known. */
export function fileDimensions(f: Pick<LibraryFile, 'width' | 'height' | 'pages'>): string {
	if (f.width && f.height) return `${f.width} × ${f.height}`;
	if (f.pages) return `${f.pages} ${f.pages === 1 ? 'page' : 'pages'}`;
	return '';
}

/** Where a file lives, in words. */
export function locationLabel(f: Pick<LibraryFile, 'area' | 'team' | 'owner' | 'former_owner'>): string {
	switch (f.area) {
		case 'org':
			return f.former_owner ? `Organisation (from ${f.former_owner})` : 'Organisation';
		case 'shared':
			return 'Shared';
		case 'team':
			return f.team?.name ?? 'Team';
		default:
			return f.owner ? `${f.owner.username}’s files` : 'Personal';
	}
}

/** Downloads a library file's bytes (from this origin) as a File, e.g. to crop it into a new copy. */
export async function fetchFileContent(f: Pick<LibraryFile, 'id' | 'name' | 'content_type'>): Promise<File> {
	const res = await fetch(apiUrl(`/api/me/files/${encodeURIComponent(f.id)}/content`), { credentials: 'include' });
	if (!res.ok) {
		if (res.status === 401) session.expire();
		const body = await res.json().catch(() => null);
		throw new ApiError(
			(body && typeof body.error === 'string' && body.error) || 'Could not open that file',
			res.status
		);
	}
	return new File([await res.blob()], f.name, { type: f.content_type });
}
