import type { Connection, Field, Manifest, SettingsChange } from './types';

/** Form state for a connection's settings: every non-secret field, plus secrets being typed. */
export interface FormValues {
	config: Record<string, string | boolean>;
	secrets: Record<string, string>;
}

/** The form's starting values: the saved settings, or each field's default for a new connection. */
export function initialValues(fields: Field[], saved?: Pick<Connection, 'config'>): FormValues {
	const config: FormValues['config'] = {};
	const secrets: FormValues['secrets'] = {};
	for (const f of fields) {
		if (f.type === 'secret') {
			secrets[f.key] = '';
			continue;
		}
		const raw = saved ? saved.config[f.key] : f.default;
		config[f.key] = f.type === 'bool' ? raw === true : typeof raw === 'string' ? raw : '';
	}
	return { config, secrets };
}

/**
 * What to send for the form: on a new connection, everything filled in; on
 * a saved one, only what changed. Secrets left blank keep their saved value.
 */
export function settingsChange(
	fields: Field[],
	values: FormValues,
	saved?: Pick<Connection, 'config'>
): SettingsChange {
	const out: SettingsChange = { config: {}, secrets: {} };
	for (const f of fields) {
		if (f.type === 'secret') {
			const v = values.secrets[f.key]?.trim() ?? '';
			if (v) out.secrets[f.key] = v;
			continue;
		}
		const v = values.config[f.key];
		const value = typeof v === 'string' ? v.trim() : v;
		if (!saved) {
			if (value !== '' && value !== undefined) out.config[f.key] = value;
			continue;
		}
		const before = saved.config[f.key] ?? (f.type === 'bool' ? false : '');
		if (value !== before) out.config[f.key] = value;
	}
	return out;
}

export function isEmptyChange(c: SettingsChange): boolean {
	return Object.keys(c.config).length === 0 && Object.keys(c.secrets).length === 0;
}

/**
 * Problems the browser can spot before saving, by field key. The server
 * checks everything again (and more, like private addresses).
 */
export function fieldProblems(
	fields: Field[],
	values: FormValues,
	saved?: Pick<Connection, 'secrets'>
): Record<string, string> {
	const problems: Record<string, string> = {};
	for (const f of fields) {
		if (f.type === 'secret') {
			if (f.required && !values.secrets[f.key]?.trim() && !saved?.secrets[f.key]) {
				problems[f.key] = `Enter the ${f.label.toLowerCase()}`;
			}
			continue;
		}
		const v = values.config[f.key];
		if (f.type === 'bool') continue;
		const s = typeof v === 'string' ? v.trim() : '';
		if (!s) {
			if (f.required) problems[f.key] = `Enter the ${f.label.toLowerCase()}`;
			continue;
		}
		if (f.type === 'url' && !/^https?:\/\/[^\s/]+/i.test(s)) {
			problems[f.key] = 'Enter a full URL starting with https://';
		}
		if (f.type === 'select' && !f.options?.some((o) => o.value === s)) {
			problems[f.key] = `Choose one of the options`;
		}
	}
	return problems;
}

/** A random value for a field with `generate`: 32 bytes as hex. */
export function generateSecret(): string {
	const bytes = new Uint8Array(32);
	crypto.getRandomValues(bytes);
	return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
}

/** Whether a manifest lets this user add a connection for the given scope. */
export function canConnect(m: Pick<Manifest, 'scopes'>, scope: 'org' | 'user', isAdmin: boolean): boolean {
	return m.scopes.includes(scope) && (scope === 'user' || isAdmin);
}
