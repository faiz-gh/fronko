import { describe, expect, it } from 'vitest';
import { canConnect, fieldProblems, generateSecret, initialValues, isEmptyChange, settingsChange } from './fields';
import type { Field } from './types';

const fields: Field[] = [
	{ key: 'url', label: 'Payload URL', type: 'url', required: true },
	{ key: 'token', label: 'API token', type: 'secret', required: true },
	{ key: 'mode', label: 'Mode', type: 'select', options: [{ value: 'a', label: 'A' }], default: 'a' },
	{ key: 'notify', label: 'Notify', type: 'bool' },
	{ key: 'note', label: 'Note', type: 'textarea' }
];

describe('initialValues', () => {
	it('uses defaults for a new connection', () => {
		expect(initialValues(fields)).toEqual({
			config: { url: '', mode: 'a', notify: false, note: '' },
			secrets: { token: '' }
		});
	});

	it('uses the saved settings, never secrets', () => {
		const saved = { config: { url: 'https://x.io', notify: true } };
		expect(initialValues(fields, saved)).toEqual({
			config: { url: 'https://x.io', mode: '', notify: true, note: '' },
			secrets: { token: '' }
		});
	});
});

describe('settingsChange', () => {
	it('sends everything filled in for a new connection', () => {
		const values = initialValues(fields);
		values.config.url = ' https://x.io/hook ';
		values.secrets.token = ' abc ';
		expect(settingsChange(fields, values)).toEqual({
			config: { url: 'https://x.io/hook', mode: 'a', notify: false },
			secrets: { token: 'abc' }
		});
	});

	it('sends only what changed on a saved one', () => {
		const saved = { config: { url: 'https://x.io', mode: 'a' } };
		const values = initialValues(fields, saved);
		expect(isEmptyChange(settingsChange(fields, values, saved))).toBe(true);

		values.config.note = 'hello';
		values.config.url = '';
		expect(settingsChange(fields, values, saved)).toEqual({ config: { note: 'hello', url: '' }, secrets: {} });
	});

	it('leaves blank secrets alone', () => {
		const saved = { config: {} };
		const values = initialValues(fields, saved);
		values.secrets.token = '   ';
		expect(settingsChange(fields, values, saved).secrets).toEqual({});
	});
});

describe('fieldProblems', () => {
	it('asks for required fields', () => {
		expect(fieldProblems(fields, initialValues(fields))).toEqual({
			url: 'Enter the payload url',
			token: 'Enter the api token'
		});
	});

	it('accepts a saved secret left blank', () => {
		const values = initialValues(fields);
		values.config.url = 'https://x.io';
		expect(fieldProblems(fields, values, { secrets: { token: true } })).toEqual({});
	});

	it('checks URLs and options', () => {
		const values = initialValues(fields);
		values.config.url = 'x.io/hook';
		values.config.mode = 'z';
		values.secrets.token = 't';
		const problems = fieldProblems(fields, values);
		expect(Object.keys(problems).sort()).toEqual(['mode', 'url']);
	});
});

describe('helpers', () => {
	it('generates 32-byte hex secrets', () => {
		const a = generateSecret();
		expect(a).toMatch(/^[0-9a-f]{64}$/);
		expect(generateSecret()).not.toBe(a);
	});

	it('lets members connect only for themselves', () => {
		const both = { scopes: ['org', 'user'] as ('org' | 'user')[] };
		expect(canConnect(both, 'org', false)).toBe(false);
		expect(canConnect(both, 'user', false)).toBe(true);
		expect(canConnect(both, 'org', true)).toBe(true);
		expect(canConnect({ scopes: ['org'] }, 'user', true)).toBe(false);
	});
});
