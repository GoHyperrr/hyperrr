# Contributing to hyperrr

## Git Workflow

### Branching Strategy
- `main`: Production-ready code.
- `feat/*`: New features.
- `fix/*`: Bug fixes.
- `docs/*`: Documentation changes.
- `refactor/*`: Code refactoring.

### Commit Messages
We follow [Conventional Commits](https://www.conventionalcommits.org/):
- `feat: ...`
- `fix: ...`
- `docs: ...`
- `style: ...`
- `refactor: ...`
- `test: ...`
- `chore: ...`

### Development Process
1. Create a new branch from `main`.
2. Make your changes.
3. Ensure tests pass (`make test`) and linting is clean (`make lint`).
4. Maintain 95%+ code coverage (`make coverage`).
5. Submit a Pull Request.

## Tools
- `go`: 1.22+
- `golangci-lint`: for linting.
- `gotestsum`: for formatted test output.
- `lefthook`: for pre-commit hooks.
