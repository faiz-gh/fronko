/** A 12-character temporary password without look-alikes (0/O, 1/l/I), easy to read out or type. */
export function generatePassword(): string {
	const alphabet = 'abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789';
	const bytes = crypto.getRandomValues(new Uint8Array(12));
	return Array.from(bytes, (b) => alphabet[b % alphabet.length]).join('');
}

/** Clipboard text for handing someone their sign-in details. */
export function signInDetails(username: string, password: string): string {
	return `Sign in to Fronko at ${location.origin}/login\nUsername: ${username}\nTemporary password: ${password}`;
}
