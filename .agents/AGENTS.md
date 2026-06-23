<RULE[semantic_versioning]>
# Semantic Versioning & Release Rules

This project adheres to [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html).

When asked to update the version, prepare a release, or update the changelog, you MUST follow these rules:

1. **Versioning Scheme**: Given a version number `MAJOR.MINOR.PATCH`, increment the:
   - **MAJOR** version when you make incompatible API changes (breaking changes),
   - **MINOR** version when you add functionality in a backward-compatible manner (new features), and
   - **PATCH** version when you make backward-compatible bug fixes.

2. **Changelog Updates**:
   - Always update `CHANGELOG.md` before releasing a new version.
   - Use the exact format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
   - Add a new `## [MAJOR.MINOR.PATCH] - YYYY-MM-DD` header.
   - Categorize changes into `### Added`, `### Changed`, `### Deprecated`, `### Removed`, `### Fixed`, or `### Security`.

3. **Release Process**:
   - Create a new branch for the release (e.g., `release/vX.Y.Z`).
   - Commit the `CHANGELOG.md` updates with message format `chore: release vX.Y.Z`.
   - Create an annotated Git tag `vX.Y.Z` pointing to the release commit.
   - Push the branch and the tag to GitHub.
   - Use the `gh` CLI to create the release: `gh release create vX.Y.Z -F CHANGELOG.md -t "Release vX.Y.Z"`.
   - Build and push the corresponding Docker images with tags `latest` and `vX.Y.Z` to `ghcr.io`.

</RULE[semantic_versioning]>
