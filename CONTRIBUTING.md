# Contributing to CoreDNS-GSLB

Contributions are welcome and greatly appreciated! Whether you are fixing a bug, improving documentation, adding a feature, or enhancing tests.

To make the contribution process as smooth as possible, our development guide has been organized into dedicated sections:

## 📚 Development Guides

*   **[Contribution Guidelines](docs/developer/contributing.md)**: Rules for backward compatibility, git commit messages, linters, and updating dependencies.
*   **[Development Environment Setup](docs/developer/dev_environment.md)**: How to run and test failovers, GeoIP, and ECS features locally using Docker Compose.
*   **[Binary Compilation](docs/developer/compilation.md)**: How to register the GSLB plugin in CoreDNS and compile the binary.
*   **[Running Tests](docs/developer/testing.md)**: Executing unit and integration tests locally, and troubleshooting port conflicts.

---

## 🚀 Quick Rules

1.  **Backward Compatibility**: Keep the project backward compatible and follow standard Go conventions.
2.  **Test Coverage**: Add unit tests for all bug fixes and new features. Ensure the build and lint checks pass cleanly.
3.  **Clean Commits**: Keep commit histories clean and squash your commits before opening a pull request.