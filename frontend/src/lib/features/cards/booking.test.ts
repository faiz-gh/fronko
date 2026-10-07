import { describe, expect, it } from 'vitest';
import { bookingHref } from './card';

describe('bookingHref', () => {
	const calendly = { url: 'https://calendly.com/ada/30min', prefill: { name: 'name', email: 'email' } };

	it('fills in what the visitor gave the lead form', () => {
		const href = bookingHref(calendly, { name: 'Grace Hopper', email: 'grace@example.com' });
		const url = new URL(href!);
		expect(url.searchParams.get('name')).toBe('Grace Hopper');
		expect(url.searchParams.get('email')).toBe('grace@example.com');
	});

	it('splits the name for pages that want first and last names', () => {
		const hubspot = {
			url: 'https://meetings.hubspot.com/ada?uuid=1',
			prefill: { firstName: 'first_name', lastName: 'last_name', email: 'email' }
		};
		const url = new URL(bookingHref(hubspot, { name: 'Grace Brewster Hopper', email: 'g@x.com' })!);
		expect(url.searchParams.get('firstName')).toBe('Grace');
		expect(url.searchParams.get('lastName')).toBe('Brewster Hopper');
		expect(url.searchParams.get('uuid')).toBe('1');
	});

	it('leaves the link alone without a visitor or prefill, and refuses unsafe links', () => {
		expect(bookingHref(calendly, null)).toBe('https://calendly.com/ada/30min');
		expect(bookingHref({ url: 'https://cal.com/ada' }, { name: 'G', email: 'g@x.com' })).toBe('https://cal.com/ada');
		expect(bookingHref({ url: 'javascript:alert(1)' }, null)).toBeNull();
	});
});
