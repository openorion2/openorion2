# Releases and updates

## What a release is

A GitHub release of `openorion2/openorion2`, tagged `vX.Y.Z`, with these
assets:

- one binary per platform, `openorion2-<goos>-<goarch>` (`.exe` on
  Windows): darwin, windows and linux on amd64 and arm64;
- `SHA256SUMS`: their checksums, one `checksum  name` line each;
- `SHA256SUMS.sig`: the ed25519 signature of `SHA256SUMS`, base64, made
  with the release key.

The release key's public half is built into the game
(`internal/update.PublicKey`); the private half stays with the maintainer
(`~/.config/openorion2/release-ed25519.key`). Keep a backup of it: without
it no release can be signed for the games already installed, and a new key
reaches only the games installed after it.

## Making one

```
go run ./tools/release build dist v0.3.0
go run ./tools/release sign dist ~/.config/openorion2/release-ed25519.key
gh release create v0.3.0 dist/* --repo openorion2/openorion2 --title v0.3.0 --notes "..."
```

`build` sets the version into the binaries (`-ldflags -X
.../internal/update.Version=v0.3.0`); `sign` refuses a key other than the
built-in one.

## How the game updates

At the start a release build (not `dev`, not `-headless`, not
`-noupdate`) asks GitHub for the latest release in the background. If it
is newer and has this platform's binary, the main menu offers it once. Yes
downloads the binary, checks `SHA256SUMS.sig` against the built-in key and
the binary against `SHA256SUMS`, writes it next to the running one and
renames it over it (Windows: the running `.exe` is renamed to `.old` first
and removed at the next start), then the game starts the new binary with
the same arguments. If the directory is not writable the game says so and
the player downloads the new version by hand.
