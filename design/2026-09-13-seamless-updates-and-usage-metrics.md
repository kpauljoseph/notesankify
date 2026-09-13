# Seamless updates and usage metrics

Date: 2026-09-13
Status: approved design, awaiting implementation plan
Target release: v1.1.0

## Goal

v1.1.0 is the last update users install by hand. From then on the app
updates itself on macOS and Windows with one click, and the project can
see how many installs are in regular use, without collecting anything that
identifies a person.

## Scope of v1.1.0

Everything below ships together, because users should be asked to update
by hand exactly once.

1. macOS folder-prompt fix (branch `fix/macos-folder-dialog-prompts`, done).
2. version.json published on release publish, not on tag push
   (branch `ci/publish-version-on-release`, done).
3. One-click updater for macOS and Windows; Linux keeps the download link.
4. Ad-hoc code signing of the macOS bundle with a stable identifier
   requirement, so an update does not reset folder permissions.
5. Usage counting: Cloudflare in front of notesankify.com, a Worker on the
   version.json route, and a descriptive User-Agent from the app.
6. `RELEASING.md`, the written release checklist.
7. Documentation touches: the `~/NotesAnkify` folder, one privacy-page
   sentence about the daily version check.

## Out of scope

- Developer ID signing and notarization (issue 21). The signing command is
  designed so only the identity changes later.
- One-click updates on Linux. The tarball installs to `/usr/local/bin`,
  which needs sudo.
- A stats web page. Queries run from the command line first.
- Native folder pickers.
- Background download on metered connections, delta updates, rollback UI.

## Decisions already made

| Decision | Choice |
|---|---|
| Platforms with one-click update | macOS and Windows. Linux gets the link. |
| Update UX | Silent download and verify, then "Update ready. Restart now / Later". |
| Updater architecture | Own small updater on top of minio/selfupdate, discovery via version.json. |
| Signature format | minisign (Ed25519). |
| Metrics host | Cloudflare free tier, Worker plus D1, in front of GitHub Pages. |
| Spec location | `design/`, because `docs/` is published to the website. |

## Architecture overview

```
GitHub release assets            notesankify.com (GitHub Pages behind Cloudflare)
  DMG, zips, tar.xz                 version.json  <-- publish-version.yml on release publish
  *.minisig signatures                   |
        ^                                | Worker: proxy + count into D1
        |  download payload + .minisig   v
   NotesAnkify app  ---- GET version.json (User-Agent: NotesAnkify/1.1.0 (darwin; arm64))
        |
   updater.Manager: check -> stage -> verify -> ready -> apply -> relaunch
```

## 1. Release artifacts and signing

### Artifacts per release

| Asset | Purpose | New? |
|---|---|---|
| `NotesAnkify-macos-universal-vX.dmg` | First-time macOS install | existing |
| `NotesAnkify-macos-universal-vX.zip` | macOS update payload: the signed `.app`, zipped with `ditto -c -k --keepParent` | new |
| `NotesAnkify-windows-amd64-vX.zip`, `NotesAnkify-windows-arm64-vX.zip` | First-time install and update payload. Each contains only `NotesAnkify.exe`. | existing |
| `NotesAnkify-linux-amd64-vX.tar.xz` | Linux install | existing |
| `<payload>.minisig` for the macOS zip and both Windows zips | Detached minisign signatures | new |

### Signing key

- Generated once on the maintainer's Mac with `minisign -G`. Private key at
  `~/.minisign/minisign.key`, passphrase protected. A copy of the key and
  passphrase goes into the maintainer's password manager. Losing the key
  means no existing install can auto-update again; a new key requires one
  more manual install by every user.
- The public key string is a constant in `pkg/updater/pubkey.go`.
- Each payload is signed with `minisign -Sm <payload>`, producing
  `<payload>.minisig`. The signature covers the whole file, so nothing is
  extracted from a payload before the signature verifies.

### macOS bundle signing

Before the DMG and zip are built:

```
codesign --force --sign - --identifier com.notesankify.app \
  -r '=designated => identifier "com.notesankify.app"' NotesAnkify.app
```

This gives the bundle a designated requirement that does not change per
build, so macOS keeps the user's folder permissions across updates. The
x86_64 slice, unsigned today, becomes signed as a side effect. When a
Developer ID exists, `--sign -` becomes the identity name, plus
`--options runtime --timestamp`, and notarization follows; nothing else in
the pipeline changes.

### Build tooling changes

- Makefile: new targets `darwin-sign` (codesign as above, runs after
  `darwin-universal`), `darwin-zip` (ditto zip of the signed bundle),
  `sign-payloads` (minisign over the three payloads, prompts once for the
  passphrase). `darwin-dmg` depends on `darwin-sign`. `package-all` runs
  all of them.
- `upload-release.sh` uploads the macOS zip and the three `.minisig` files
  in addition to today's assets.
- `bundle-assets` runs `fyne bundle` from inside `assets/bundle`, because
  the current fyne CLI emits a go:embed path relative to the package
  directory. This is what already works by accident with the older CLI on
  the maintainer's Mac and will keep working after an upgrade.

## 2. The manifest: version.json

Backward compatible. 1.0.0 clients keep reading `latest_version`,
`platform_downloads`, `update_message` and `force_update`, so the one
manual upgrade works through the existing dialog. One block is added:

```json
{
  "latest_version": "v1.1.0",
  "min_version": "v1.1.0",
  "update_message": "New version v1.1.0 is available with improvements and bug fixes!",
  "force_update": false,
  "platform_downloads": {
    "windows/amd64": "https://github.com/kpauljoseph/notesankify/releases/download/v1.1.0/NotesAnkify-windows-amd64-v1.1.0.zip",
    "windows/arm64": "https://github.com/kpauljoseph/notesankify/releases/download/v1.1.0/NotesAnkify-windows-arm64-v1.1.0.zip",
    "darwin/all":    "https://github.com/kpauljoseph/notesankify/releases/download/v1.1.0/NotesAnkify-macos-universal-v1.1.0.dmg",
    "linux/amd64":   "https://github.com/kpauljoseph/notesankify/releases/download/v1.1.0/NotesAnkify-linux-amd64-v1.1.0.tar.xz"
  },
  "update_packages": {
    "darwin/all":    {"url": ".../NotesAnkify-macos-universal-v1.1.0.zip", "signature_url": ".../NotesAnkify-macos-universal-v1.1.0.zip.minisig"},
    "windows/amd64": {"url": ".../NotesAnkify-windows-amd64-v1.1.0.zip",   "signature_url": ".../NotesAnkify-windows-amd64-v1.1.0.zip.minisig"},
    "windows/arm64": {"url": ".../NotesAnkify-windows-arm64-v1.1.0.zip",   "signature_url": ".../NotesAnkify-windows-arm64-v1.1.0.zip.minisig"}
  }
}
```

- Linux is absent from `update_packages` on purpose. A platform without an
  entry gets the link dialog.
- `publish-version.yml` generates the block from the tag with URL patterns.
  It needs no access to built files and no hashes, because trust comes
  from the signatures next to the assets.
- The JSON itself is not signed. Nothing in it can make the app run
  unsigned code; the worst a tampered manifest can do is point at a payload
  that fails verification.
- The platform key is `runtime.GOOS + "/" + runtime.GOARCH`, except macOS
  which is always `darwin/all` (universal binary), matching today's code.

### Version comparison

`compareVersions` compares strings today, so 1.10.0 sorts below 1.9.0. The
updater parses with `golang.org/x/mod/semver`. Versions are normalised to a
leading `v`. A compiled version that is not a valid release tag (for
example `dev` or `v1.0.0-2-gc638a67`) is a developer build: the app still
checks and shows the link dialog, but never stages or applies a package.

## 3. In-app update flow

### Schedule

Unchanged: a check five seconds after launch, then every 24 hours,
throttled to once an hour. The launch-time check doubles as the daily
usage signal.

### Manager

`pkg/updater` gains a `Manager` with a small state machine:

```
idle -> checking -> (up to date) -> idle
                 -> (link only)  -> notify(link)
                 -> downloading -> verifying -> ready -> applying -> relaunch
                                             \-> failed -> notify(link)
```

- Runs entirely off the UI goroutine. Reports transitions through one
  callback; the GUI wraps that callback in `fyne.Do`.
- Exactly one check or download at a time, guarded by a mutex. A second
  call while one is in flight is a no-op.
- Downloads run under a context cancelled when the app quits.

### Steps

1. Check. Fetch the manifest. Compare versions. If not newer, idle. If
   newer but no package for this platform, or the platform is not
   in-place updatable (section 4), or automatic downloads are switched
   off (section 5), report `link` with the download URL and stop.
2. Stage. Download the payload and its `.minisig` into
   `<user cache dir>/NotesAnkify/updates/<version>/`. Files are written to
   a temporary name and renamed on completion. If a staged payload for the
   same version already exists and verifies, it is reused without a
   download. Staged folders for other versions are deleted.
3. Verify. Check the minisign signature over the whole payload with the
   compiled-in public key. Then open the zip and confirm it contains the
   expected entry (`NotesAnkify.app/Contents/MacOS/notesankify-gui` and
   `NotesAnkify.app/Contents/Info.plist` on macOS, `NotesAnkify.exe` on
   Windows). Only then report `ready`.
4. Offer. The GUI shows "NotesAnkify <version> is ready to install", the
   update message, and buttons Restart now and Later. `force_update` hides
   Later. The GUI defers the dialog while a PDF batch is running and shows
   it when processing finishes. Later means nothing more happens until the
   next launch, when the staged payload is found, re-verified and offered
   again.
5. Apply on Restart now.
   - Windows: extract `NotesAnkify.exe` from the zip and pass it to
     `selfupdate.Apply` (minio/selfupdate v0.6.0) with `TargetPath` set to
     the running executable. The library renames the running exe aside,
     writes the new one, and rolls back on failure.
   - macOS: expand the zip with `/usr/bin/ditto -x -k` into the installed
     app's parent folder under a hidden temporary name, then two renames:
     `NotesAnkify.app` becomes `NotesAnkify.app.old`, the temporary name
     becomes `NotesAnkify.app`. Same-folder renames are atomic and keep
     the whole bundle, plist, icon and signature intact. On any failure
     the renames are reversed.
6. Relaunch. Start the new build (`open -n <bundle>` on macOS, a detached
   process on Windows) and exit. On its first start the new build removes
   the leftover (`NotesAnkify.app.old`, or the hidden old exe on Windows),
   and only if it is not itself running from that leftover.

### Environment override for rehearsal

`NOTESANKIFY_UPDATE_URL` replaces the manifest URL when set. Draft-release
assets are not publicly downloadable, so the release rehearsal serves a
hand-written manifest and freshly built payloads from a local folder with a
one-line HTTP server and launches the previous version with the override.

## 4. Failure handling and safety

- In-place checks, before any download:
  - macOS: the running executable path must not be under `/Volumes/`
    (running from the DMG) or contain `/AppTranslocation/`, and the app's
    parent folder must be writable. On failure the link dialog adds the
    line "Move NotesAnkify to your Applications folder to enable automatic
    updates."
  - Windows: the exe's folder must be writable.
- Disk space: free space in both the staging and install folders must
  exceed three times the payload size, else the link fallback.
- Downloads use their own HTTP client with a per-read idle timeout and no
  overall timeout (the existing ten-second client cannot fetch 45 MB), a
  200 MB size cap, and a context tied to app lifetime. A failed download is
  deleted and retried at the next scheduled check.
- Verification failure deletes the staged files, logs the reason, and
  falls back to the link. A bad signature is not retried.
- Apply failure reverses the renames and shows the link. The previous
  build is never deleted until the new one has started successfully.
- Every transition is written to the existing log file.

## 5. App identity and preferences

- The GUI moves from `app.New()` to `app.NewWithID("com.notesankify.app")`.
  Fyne preferences then persist in its per-app preferences file. Visible
  side effects: the folder dialog remembers the last folder, and it shows
  an "App Files" favourite for Fyne's storage folder. Both are acceptable.
- One new preference: `updates.auto_download` (bool, default true), shown
  as the checkbox "Download updates automatically" beside the verbose
  logging checkbox. Off means: still check, still show the link dialog for
  new versions, never download.
- No other state is persisted. In particular the launch-time check is not
  suppressed, since it is the usage signal.
- Release builds are built at the tag so the compiled version is exactly
  the tag. The checklist enforces this.

## 6. Cloudflare Worker and User-Agent

### DNS

1. Add notesankify.com to a free Cloudflare account. It imports the
   existing records: the four GitHub Pages A records on the apex and the
   CNAME on www.
2. Mark both proxied. SSL/TLS mode Full.
3. Change the nameservers at Namecheap to the two Cloudflare assigns.
   Records are unchanged, so the site stays up during propagation.

### Worker

Route: `notesankify.com/version.json`.

- Proxy the request to the GitHub Pages origin with a five-minute edge
  cache and return the body and headers unchanged.
- After the response is dispatched (`ctx.waitUntil`), insert one row into
  D1 table `pings`:

  ```sql
  CREATE TABLE pings (
    day        TEXT NOT NULL,   -- YYYY-MM-DD, UTC
    visitor    TEXT NOT NULL,   -- first 16 bytes, hex, of sha256(day || ip || user_agent || salt)
    os         TEXT NOT NULL,   -- darwin, windows, linux, unknown
    arch       TEXT NOT NULL,   -- arm64, amd64, unknown
    version    TEXT NOT NULL,   -- app version or "legacy"
    client     TEXT NOT NULL,   -- "app" or "legacy"
    PRIMARY KEY (day, visitor)
  );
  ```

  `INSERT OR IGNORE`, so repeated calls from one install on one day count
  once. The IP is never stored. The salt is a Worker secret.
- Only User-Agents that look like the app are recorded: `NotesAnkify/<ver>
  (<os>; <arch>)` from 1.1.0 onward, and `Go-http-client/*` from 1.0.0,
  recorded as client `legacy`, version `legacy`. Anything else, browsers
  and crawlers, is proxied and not recorded.
- The whole handler is wrapped in try/catch. On any error it returns a
  plain `fetch(request)` of the origin, so a Worker bug cannot break update
  checks. The app additionally falls back to the GitHub API if version.json
  fails, as today.

### App side

One header on the existing request:
`User-Agent: NotesAnkify/1.1.0 (darwin; arm64)`. No install ID, no extra
fields.

### Reading the numbers

`cloudflare/stats.sql` holds queries for daily, weekly and monthly active
installs, by OS and by version. Run with `wrangler d1 execute` from the
maintainer's Mac. A protected stats page is a possible later addition.

### Repository layout and free-tier fit

`cloudflare/` contains `worker.js`, `wrangler.toml`, `schema.sql`,
`stats.sql`, and a `README.md` with the click-by-click setup and deploy
steps. Expected volume is on the order of ten thousand requests a day
against free-tier limits of one hundred thousand Worker requests and one
hundred thousand D1 row writes a day.

### Privacy page

One sentence: the app checks notesankify.com once a day for updates; the
request carries the app version and operating system; these requests are
counted to estimate how many people use NotesAnkify; no identifier is
stored.

## 7. Release checklist (`RELEASING.md`)

One-time: Docker, fyne-cross, the fyne CLI, minisign and the key pair, the
gh CLI logged in, wrangler logged in for Worker changes.

Per release:

1. Merge to main. Rename `[Unreleased]` in `CHANGELOG.md` to the version
   and date; the workflow extracts notes by that heading.
2. `git tag vX.Y.Z && git push origin vX.Y.Z`. The workflow creates the
   draft release with notes.
3. `git checkout vX.Y.Z && make package-all`. Builds, signs the macOS
   bundle, zips it, signs the payloads (one passphrase prompt).
4. `GITHUB_TOKEN=$(gh auth token) ./upload-release.sh vX.Y.Z`.
5. Smoke test the DMG from the draft release on the Mac: version shown in
   the header, `codesign -d -r-` shows the identifier requirement, Browse
   opens without prompts.
6. Rehearse the update path with `NOTESANKIFY_UPDATE_URL` against a local
   manifest and the built payloads, from the previous version.
7. Publish the release. `publish-version.yml` updates version.json; confirm
   with curl after a few minutes (five-minute edge cache).
8. Next day, run `stats.sql` and watch the new version's share.

## 8. Testing

Unit tests in Ginkgo, pure Go, runnable on Linux:

- Version comparison, including 1.10 versus 1.9 and developer builds.
- Manifest parsing, platform key selection, link fallback when a platform
  has no package.
- Signature verification with `aead.dev/minisign`, which signs and
  verifies, so tests generate a throwaway key pair and sign in memory.
- Staging: temporary name then rename, reuse of a verified payload,
  cleanup of other versions, the size cap.
- In-place checks with the filesystem probes behind a small interface.
- Bundle swap and rollback on fake bundle directories, with extraction
  behind an interface: Go's zip reader in tests, `ditto` on macOS.
- Manager state machine against an `httptest` server serving manifest,
  payload and signature: happy path and each failure branch.

Continuous integration: `ci.yml` runs `go test ./pkg/updater/...` on
GitHub's Linux, macOS and Windows runners on every push. The updater has
no GUI dependency, so it needs none of the cgo setup that stalled the old
build workflow. This gives the Windows exe swap a real run on Windows on
every change.

Manual before v1.1.0 ships: the rehearsal on the maintainer's Mac,
including a check that folder permissions survived the swap, plus one
Windows run if a machine is available. The Worker gets a `wrangler dev`
session and a unit test for the User-Agent parser.

## Dependencies added

| Module | Use |
|---|---|
| `github.com/minio/selfupdate` v0.6.0 | Windows executable swap with rollback |
| `aead.dev/minisign` | Signature verification in the app, signing in tests |
| `golang.org/x/mod` | `semver` package for version comparison |

## Work packages

1. Signing and packaging: Makefile targets, upload script, minisign key,
   `bundle-assets` fix.
2. Manifest: `publish-version.yml` block, `VersionResponse` fields, semver.
3. Updater package: manager, staging, verification, per-OS apply,
   relaunch, override, tests, `ci.yml`.
4. GUI wiring: `NewWithID`, preference and checkbox, ready dialog, deferral
   while processing, leftover cleanup at startup, User-Agent.
5. Cloudflare: Worker, D1 schema, queries, README, DNS steps.
6. Documents: `RELEASING.md`, changelog, docs and privacy sentence.
