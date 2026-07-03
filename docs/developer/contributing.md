# Contribution Guidelines

Contributions are welcome and appreciated! Whether it's fixing a bug, improving documentation, adding a feature, or enhancing tests.

Before opening a pull request, please read the following guidelines to ensure smooth collaboration.

## Contribution Rules

1. **Backward Compatibility**: Keep the project backward compatible and follow existing Go coding conventions.
2. **Tests**: Add unit tests for any new features, bug fixes, or important logic changes. Ensure all existing tests continue to pass.
3. **Commit Messages**: Use descriptive, meaningful commit messages. Clean up/squash your commit history before submitting your PR.
4. **Documentation**: Document any configuration flags, API changes, or new behavior in the corresponding `docs/` files.

## Running Linters

We use `golangci-lint` to maintain code quality. Please run it locally before pushing your changes.

### Installing Linters

*   **Debian/Ubuntu**:
    ```bash
    sudo apt install build-essential
    ```
*   **RHEL/CentOS**:
    ```bash
    sudo dnf group install c-development
    ```

Install the Go linter utility:
```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Executing the Linter

Run the linter via the Makefile wrapper:
```bash
make lint
```

## Updating CoreDNS Dependencies

If you need to update CoreDNS or its DNS dependency packages:

```bash
go mod edit -go=1.25
go get github.com/coredns/coredns@v1.14.4
go get github.com/miekg/dns@v1.1.72
go mod tidy
```

## Running the Documentation Site Locally

To preview your documentation changes locally, run the integrated static web server:

```bash
make docs-serve
```
This serves the documentation locally, allowing you to verify navigation and formatting before submitting PRs.
