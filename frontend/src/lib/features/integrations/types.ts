// Mirrors backend/internal/integrations: the catalog is driven entirely by
// each provider's manifest, so a new provider needs no frontend change
// beyond (optionally) a logo in registry.ts.

export type Category = 'lead_sync' | 'calendar' | 'directory' | 'sso';
export type Scope = 'org' | 'user';
export type AuthKind = 'none' | 'api_key' | 'oauth2' | 'link' | 'saml' | 'scim_token';
export type Availability = 'available' | 'beta' | 'coming_soon';
export type FieldType = 'text' | 'url' | 'secret' | 'select' | 'textarea' | 'bool';
export type ConnectionStatus = 'pending' | 'active' | 'error';

export interface FieldOption {
	value: string;
	label: string;
}

/** One setting on a connection, as the provider's manifest describes it. */
export interface Field {
	key: string;
	label: string;
	type: FieldType;
	required?: boolean;
	help?: string;
	placeholder?: string;
	options?: FieldOption[];
	default?: unknown;
	/** Offer to generate a random value (a secret both sides need to know). */
	generate?: boolean;
}

export interface Manifest {
	id: string;
	name: string;
	category: Category;
	description: string;
	scopes: Scope[];
	auth: AuthKind;
	status: Availability;
	multiple: boolean;
	fields: Field[];
	setup_steps?: string[];
	docs_url?: string;
	keywords?: string[];
}

export interface ConnectionSummary {
	id: number;
	name: string;
	scope: Scope;
	status: ConnectionStatus;
	enabled: boolean;
}

export interface CatalogEntry extends Manifest {
	/** Why it can't be connected on this server, or ''. */
	unavailable: string;
	connections: ConnectionSummary[];
}

export interface CategoryInfo {
	id: Category;
	label: string;
	description: string;
}

export interface Catalog {
	categories: CategoryInfo[];
	providers: CatalogEntry[];
	/** What to register as the redirect URL in an OAuth app; '' without PUBLIC_URL. */
	oauth_redirect_url: string;
}

export interface UserRef {
	id: number;
	username: string;
}

export interface Connection {
	id: number;
	provider: string;
	category: Category;
	name: string;
	enabled: boolean;
	status: ConnectionStatus;
	scope: Scope;
	config: Record<string, unknown>;
	/** Which secret fields have a saved value; the values never leave the server. */
	secrets: Record<string, boolean>;
	authorized: boolean;
	/** Labels of required settings still to fill in. */
	missing: string[];
	last_error: string | null;
	last_error_at: string | null;
	failure_count: number;
	last_synced_at: string | null;
	created_by: UserRef | null;
	created_at: string;
	updated_at: string;
}

export type ActivityOutcome = 'success' | 'retrying' | 'failed';

export interface Activity {
	id: number;
	kind: 'push_lead' | 'test' | 'setup';
	outcome: ActivityOutcome;
	summary: string;
	detail?: Record<string, unknown>;
	lead_id: number | null;
	attempt: number | null;
	user: UserRef | null;
	created_at: string;
}

export interface TestResult {
	ok: boolean;
	summary: string;
	detail?: Record<string, unknown>;
}

/** What a create or update sends: only the settings being changed. */
export interface SettingsChange {
	config: Record<string, unknown>;
	secrets: Record<string, string>;
}
