// The modules under test read a few browser globals; Node has none of them.
Object.assign(globalThis, {
	window: globalThis,
	location: new URL('https://cards.example.com/')
});
