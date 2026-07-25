# Releasing GinKit

GinKit releases are published from the `main` branch through GitHub Actions. The
release workflow is tag-driven and does not require a long-lived signing key.

## Version policy

GinKit follows Semantic Versioning while the project is in the `v0.x` series:

- `v0.1.0` is the first public release.
- Minor versions may introduce new CLI commands, template features, or supported
  integrations.
- Patch versions contain backwards-compatible fixes.
- A prerelease uses a suffix such as `v0.2.0-rc.1`.

The source version defaults to `dev`. GoReleaser injects the release version into
the binary, so do not edit a version constant for a release.

## Maintainer release checklist

1. Merge the release changes into `main`.
2. Confirm the worktree is clean and the required CI and security checks passed.
3. Review the generated changelog and update `CHANGELOG.md`.
4. Create an annotated tag from `main`:

   ```bash
   git switch main
   git pull --ff-only
   git tag -a v0.1.0 -m "Release v0.1.0"
   git push origin v0.1.0
   ```

5. Monitor the `Release` workflow.
6. Confirm the GitHub Release contains Linux, macOS, and Windows archives for
   amd64 and arm64, plus `checksums.txt`.
7. Download one archive and verify it:

   ```bash
   sha256sum -c checksums.txt --ignore-missing
   gh attestation verify ginkit_0.1.0_linux_amd64.tar.gz -R Alfian57/ginkit
   ```

8. Verify the installed binary:

   ```bash
   ./ginkit --version
   ./ginkit new release-check \
     --non-interactive \
     --module example.com/release-check \
     --mode api \
     --database sqlite \
     --orm gorm
   ```

Only annotated `v0.x.y` tags created from `main` are accepted by the release
workflow. Do not reuse a published tag; create a new patch or prerelease tag.

## Failed releases

If a release workflow fails before publishing, fix the issue and rerun the
workflow for the same tag. If assets were already published, do not overwrite
the tag or silently replace binaries. Inspect the release, document the failure,
and publish a new patch or prerelease version when necessary.

## Repository settings

Maintainers should configure the repository with:

- a protected `main` branch with pull requests and required CI/security checks;
- a ruleset protecting `v*` tags;
- secret scanning and push protection;
- Dependabot security updates;
- no long-lived release secrets.

Artifact provenance is generated with GitHub Actions OIDC. The repository must
be public, or use a GitHub Enterprise plan that supports attestations for private
repositories.
