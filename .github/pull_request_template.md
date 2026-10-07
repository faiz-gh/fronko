## What and why

<!-- What does this change, and why? Link the issue, e.g. "Closes #123". -->

## How to review

<!-- Anything reviewers should look at closely. Add screenshots or a short recording for UI changes. -->

## Checklist

- [ ] Backend: `gofmt -l .` prints nothing, and `make lint`, `make test` and `make test-integration` pass
- [ ] Frontend: `npm run lint`, `npm run check`, `npm test` and `npm run build` pass
- [ ] Tests cover the change (bug fixes include a test that fails without the fix)
- [ ] Docs are updated (`backend/API.md`, READMEs, `docs/`)
- [ ] New migrations, environment variables or breaking changes are described above
