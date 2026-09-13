# Seamless Updates and Usage Metrics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship v1.1.0 so that macOS and Windows installs update themselves with one click from then on, Linux keeps a download link, and daily version checks are counted at the edge without identifying anyone.

**Architecture:** A pure-Go `pkg/updater` package (manifest fetch, minisign verification, staged download, per-OS in-place apply, relaunch) driven by a small state machine that the Fyne GUI observes. The release pipeline signs the macOS bundle with a stable ad-hoc identifier requirement, zips it, and signs every update payload with minisign. A Cloudflare Worker proxies `version.json` from GitHub Pages and records one anonymous row per install per day in D1.

**Tech Stack:** Go 1.23 (go.mod), Fyne v2.8.1, `github.com/minio/selfupdate` v0.6.0, `aead.dev/minisign` v0.3.0, `golang.org/x/mod/semver`, `golang.org/x/sys/windows`, Ginkgo v2 + Gomega, minisign CLI, Cloudflare Workers + D1 + wrangler, GitHub Actions.

**Spec:** `design/2026-09-13-seamless-updates-and-usage-metrics.md`

## Global Constraints

- Go version floor is `go 1.23.2` in go.mod. Do not raise it.
- Fyne stays at v2.8.1. UI work from goroutines goes through `fyne.Do`.
- App identifier is `com.notesankify.app`; app name `NotesAnkify`; GUI executable name `notesankify-gui`; macOS bundle `NotesAnkify.app`; Windows executable `NotesAnkify.exe`.
- Release asset names: `NotesAnkify-macos-universal-vX.Y.Z.dmg`, `NotesAnkify-macos-universal-vX.Y.Z.zip`, `NotesAnkify-windows-amd64-vX.Y.Z.zip`, `NotesAnkify-windows-arm64-vX.Y.Z.zip`, `NotesAnkify-linux-amd64-vX.Y.Z.tar.xz`, plus `<payload>.minisig` for the macOS zip and both Windows zips.
- Manifest URL `https://notesankify.com/version.json`; override env var `NOTESANKIFY_UPDATE_URL`; platform key `darwin/all` for every macOS build, otherwise `GOOS/GOARCH`.
- User-Agent format is exactly `NotesAnkify/<version without v> (<GOOS>; <GOARCH>)`.
- Preference key `updates.auto_download`, default true.
- Tests are Ginkgo v2 + Gomega, in `<package>_test` packages, one `*_suite_test.go` per package. `pkg/updater` must stay free of Fyne and cgo imports so it runs on the plain GitHub runners.
- Commit messages: conventional prefix (`feat:`, `fix:`, `ci:`, `docs:`, `build:`), imperative, signed off with `-s`, no AI attribution, no IP addresses.
- Nothing under `docs/` except user documentation: that folder is published to notesankify.com by `docs.yml`.
- Never commit `assets/bundle/` or `pkg/version/generated.go`.

## Branches and order

Part A (updater) and Part B (release tooling) both change the repo; Part C (Cloudflare) is a new folder plus one website-repo edit. Do Part A first on branch `feat/seamless-updates`, Part B on `build/release-signing`, Part C on `feat/usage-metrics`. Branches `fix/macos-folder-dialog-prompts` and `ci/publish-version-on-release` must be merged to main before Part A starts, because Part A edits the same GUI file and Part B edits `publish-version.yml`.

## File Structure

Part A, `pkg/updater` (each file one responsibility):

| File | Responsibility |
|---|---|
| `version.go` | semver normalisation and comparison |
| `types.go` (modify) | manifest and update types, add `Package` and `UpdatePackages` |
| `manifest.go` | platform key, package and download lookup |
| `fetch.go` | manifest fetch with User-Agent and env override, GitHub fallback |
| `verify.go` | minisign public key parsing and detached-signature verification |
| `payload.go` | zip layout check per OS |
| `stage.go` | download client, staging folder, size query, cleanup |
| `inplace.go`, `free_unix.go`, `free_windows.go`, `free_other.go` | install target discovery and in-place checks, disk space per OS |
| `extract.go`, `extract_darwin.go`, `extract_other.go` | zip extraction, ditto on macOS |
| `bundleswap.go` | two-rename bundle swap with rollback |
| `apply.go` | apply a payload to a bundle or an exe |
| `relaunch.go`, `relaunch_darwin.go`, `relaunch_windows.go`, `relaunch_other.go` | start the new build |
| `cleanup.go` | remove the previous build's leftovers |
| `manager.go` | state machine: check, stage, verify, ready, apply |
| `pubkey.go` | the release public key constant |
| `checker.go` (delete) | replaced by fetch.go and manager.go |

GUI: `cmd/notesankify-gui/main.go` (modify), CI: `.github/workflows/ci.yml` (create).

Part B: `Makefile`, `upload-release.sh`, `.gitignore`, `.github/workflows/publish-version.yml`, `RELEASING.md` (create), `docs/README.md`, `CHANGELOG.md`.

Part C: `cloudflare/worker.js`, `cloudflare/worker.test.mjs`, `cloudflare/wrangler.toml`, `cloudflare/schema.sql`, `cloudflare/stats.sql`, `cloudflare/README.md`; one sentence in the website repo's `privacy/index.html`.

---

# Part A: Updater

### Task 1: Version helpers and dependencies

**Files:**
- Create: `pkg/updater/version.go`
- Create: `pkg/updater/updater_suite_test.go`
- Create: `pkg/updater/version_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Produces: `func Normalize(v string) string`, `func IsRelease(v string) bool`, `func IsNewer(current, latest string) bool`

- [ ] **Step 1: Add the dependencies**

```bash
go get golang.org/x/mod@latest aead.dev/minisign@v0.3.0 github.com/minio/selfupdate@v0.6.0 golang.org/x/sys@latest
```

Expected: go.mod lists the four modules (x/sys was already indirect; it becomes direct later in Task 6).

- [ ] **Step 2: Write the suite file and the failing tests**

`pkg/updater/updater_suite_test.go`:

```go
package updater_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUpdater(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Updater Suite")
}
```

`pkg/updater/version_test.go`:

```go
package updater_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kpauljoseph/notesankify/pkg/updater"
)

var _ = Describe("Version helpers", func() {
	DescribeTable("Normalize adds the leading v",
		func(in, want string) { Expect(updater.Normalize(in)).To(Equal(want)) },
		Entry("bare", "1.2.3", "v1.2.3"),
		Entry("already prefixed", "v1.2.3", "v1.2.3"),
		Entry("whitespace", "  v1.2.3\n", "v1.2.3"),
		Entry("empty", "", ""),
	)

	DescribeTable("IsRelease accepts only tagged releases",
		func(in string, want bool) { Expect(updater.IsRelease(in)).To(Equal(want)) },
		Entry("release", "v1.1.0", true),
		Entry("release without v", "1.1.0", true),
		Entry("git describe dev build", "v1.0.0-2-gc638a67", false),
		Entry("release candidate", "v1.1.0-rc1", false),
		Entry("dev", "dev", false),
		Entry("empty", "", false),
	)

	DescribeTable("IsNewer compares numerically and ignores non-releases",
		func(current, latest string, want bool) { Expect(updater.IsNewer(current, latest)).To(Equal(want)) },
		Entry("simple", "v1.0.0", "v1.1.0", true),
		Entry("ten is newer than nine", "v1.9.0", "v1.10.0", true),
		Entry("nine is not newer than ten", "v1.10.0", "v1.9.0", false),
		Entry("equal", "v1.1.0", "v1.1.0", false),
		Entry("dev build sees the release", "v1.0.0-2-gc638a67", "v1.0.0", true),
		Entry("unparseable current never updates", "dev", "v1.1.0", false),
		Entry("latest must be a release", "v1.0.0", "v1.1.0-rc1", false),
	)
})
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./pkg/updater/`
Expected: FAIL with `undefined: updater.Normalize` (and IsRelease, IsNewer).

- [ ] **Step 4: Implement version.go**

```go
package updater

import (
	"strings"

	"golang.org/x/mod/semver"
)

// Normalize trims whitespace and guarantees the leading "v" that
// golang.org/x/mod/semver requires. An empty string stays empty.
func Normalize(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return v
}

// IsRelease reports whether v names a tagged release such as v1.2.3.
// Developer builds ("dev", "v1.0.0-2-gc638a67") and pre-releases are not.
func IsRelease(v string) bool {
	n := Normalize(v)
	return semver.IsValid(n) && semver.Prerelease(n) == "" && semver.Build(n) == ""
}

// IsNewer reports whether latest is a release newer than current. A current
// version that is not valid semver at all never sees updates, so a "dev"
// build is never nagged; a git-describe build compares as a pre-release of
// its base version and therefore sees the base release as newer.
func IsNewer(current, latest string) bool {
	l := Normalize(latest)
	if !IsRelease(l) {
		return false
	}
	c := Normalize(current)
	if !semver.IsValid(c) {
		return false
	}
	return semver.Compare(l, c) > 0
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./pkg/updater/`
Expected: PASS, 17 specs.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum pkg/updater/version.go pkg/updater/version_test.go pkg/updater/updater_suite_test.go
git commit -s -m "feat(updater): add semver-based version helpers"
```

### Task 2: Manifest types and platform lookup

**Files:**
- Modify: `pkg/updater/types.go`
- Create: `pkg/updater/manifest.go`
- Create: `pkg/updater/manifest_test.go`

**Interfaces:**
- Produces: `type Package struct { URL, SignatureURL string }`, `VersionResponse.UpdatePackages map[string]Package`, `func PlatformKey(goos, goarch string) string`, `func (v VersionResponse) PackageFor(goos, goarch string) (Package, bool)`, `func (v VersionResponse) DownloadFor(goos, goarch string) string`

- [ ] **Step 1: Write the failing tests**

`pkg/updater/manifest_test.go`:

```go
package updater_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kpauljoseph/notesankify/pkg/updater"
)

const sampleManifest = `{
  "latest_version": "v1.1.0",
  "min_version": "v1.1.0",
  "download_url": "https://notesankify.com/download",
  "update_message": "New version",
  "force_update": false,
  "platform_downloads": {
    "darwin/all": "https://example.test/NotesAnkify-macos-universal-v1.1.0.dmg",
    "linux/amd64": "https://example.test/NotesAnkify-linux-amd64-v1.1.0.tar.xz"
  },
  "update_packages": {
    "darwin/all": {"url": "https://example.test/mac.zip", "signature_url": "https://example.test/mac.zip.minisig"},
    "windows/amd64": {"url": "https://example.test/win.zip", "signature_url": ""}
  }
}`

var _ = Describe("Manifest", func() {
	var m updater.VersionResponse

	BeforeEach(func() {
		Expect(json.Unmarshal([]byte(sampleManifest), &m)).To(Succeed())
	})

	It("maps every macOS build to darwin/all", func() {
		Expect(updater.PlatformKey("darwin", "arm64")).To(Equal("darwin/all"))
		Expect(updater.PlatformKey("darwin", "amd64")).To(Equal("darwin/all"))
		Expect(updater.PlatformKey("windows", "amd64")).To(Equal("windows/amd64"))
	})

	It("returns the package for a platform that has one", func() {
		pkg, ok := m.PackageFor("darwin", "arm64")
		Expect(ok).To(BeTrue())
		Expect(pkg.URL).To(Equal("https://example.test/mac.zip"))
		Expect(pkg.SignatureURL).To(Equal("https://example.test/mac.zip.minisig"))
	})

	It("reports no package for a platform that is absent", func() {
		_, ok := m.PackageFor("linux", "amd64")
		Expect(ok).To(BeFalse())
	})

	It("reports no package when the signature URL is missing", func() {
		_, ok := m.PackageFor("windows", "amd64")
		Expect(ok).To(BeFalse())
	})

	It("prefers the platform download and falls back to download_url", func() {
		Expect(m.DownloadFor("linux", "amd64")).To(Equal("https://example.test/NotesAnkify-linux-amd64-v1.1.0.tar.xz"))
		Expect(m.DownloadFor("windows", "arm64")).To(Equal("https://notesankify.com/download"))
	})
})
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./pkg/updater/`
Expected: FAIL with `m.PackageFor undefined` and `undefined: updater.PlatformKey`.

- [ ] **Step 3: Extend types.go**

Add to `pkg/updater/types.go`, after the `VersionResponse` struct's `PlatformDownloads` field:

```go
	// UpdatePackages lists the signed in-app update payload per platform.
	// Platforms without an entry only get the download link.
	UpdatePackages map[string]Package `json:"update_packages"`
```

and add the new type at the end of the file:

```go
// Package describes a signed update payload for one platform.
type Package struct {
	URL          string `json:"url"`
	SignatureURL string `json:"signature_url"`
}
```

- [ ] **Step 4: Implement manifest.go**

```go
package updater

import "runtime"

// PlatformKey returns the key used in version.json for a platform. macOS
// ships one universal binary, so every darwin build maps to "darwin/all".
func PlatformKey(goos, goarch string) string {
	if goos == "darwin" {
		return "darwin/all"
	}
	return goos + "/" + goarch
}

// CurrentPlatformKey is PlatformKey for the running program.
func CurrentPlatformKey() string {
	return PlatformKey(runtime.GOOS, runtime.GOARCH)
}

// PackageFor returns the update payload for a platform, if the manifest
// has a complete entry for it.
func (v VersionResponse) PackageFor(goos, goarch string) (Package, bool) {
	p, ok := v.UpdatePackages[PlatformKey(goos, goarch)]
	if !ok || p.URL == "" || p.SignatureURL == "" {
		return Package{}, false
	}
	return p, true
}

// DownloadFor returns the manual download URL for a platform, falling back
// to the generic download_url.
func (v VersionResponse) DownloadFor(goos, goarch string) string {
	if u, ok := v.PlatformDownloads[PlatformKey(goos, goarch)]; ok && u != "" {
		return u
	}
	return v.DownloadURL
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./pkg/updater/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/updater/types.go pkg/updater/manifest.go pkg/updater/manifest_test.go
git commit -s -m "feat(updater): describe signed update packages in the manifest"
```

### Task 3: Manifest fetch with User-Agent, override and GitHub fallback

**Files:**
- Create: `pkg/updater/fetch.go`
- Create: `pkg/updater/fetch_test.go`

**Interfaces:**
- Consumes: `VersionResponse`, `GitHubRelease` (types.go), `version.Version` (pkg/version)
- Produces: `const DefaultManifestURL`, `const ManifestURLEnv = "NOTESANKIFY_UPDATE_URL"`, `const DefaultGitHubLatestURL`, `func ManifestURL() string`, `func UserAgent() string`, `func FetchManifest(ctx context.Context, client *http.Client, url string) (*VersionResponse, error)`, `func FetchGitHubLatest(ctx context.Context, client *http.Client, url string) (*VersionResponse, error)`

- [ ] **Step 1: Write the failing tests**

`pkg/updater/fetch_test.go`:

```go
package updater_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kpauljoseph/notesankify/pkg/updater"
)

var _ = Describe("Fetching the manifest", func() {
	It("uses the override URL when the environment variable is set", func() {
		GinkgoT().Setenv(updater.ManifestURLEnv, "http://localhost:1/version.json")
		Expect(updater.ManifestURL()).To(Equal("http://localhost:1/version.json"))
	})

	It("uses the production URL otherwise", func() {
		GinkgoT().Setenv(updater.ManifestURLEnv, "")
		Expect(updater.ManifestURL()).To(Equal(updater.DefaultManifestURL))
	})

	It("identifies the app, version and platform in the User-Agent", func() {
		ua := updater.UserAgent()
		Expect(ua).To(HavePrefix("NotesAnkify/"))
		Expect(ua).To(HaveSuffix("(" + runtime.GOOS + "; " + runtime.GOARCH + ")"))
		Expect(ua).NotTo(ContainSubstring("/v"), "version is sent without the v prefix")
	})

	It("sends the User-Agent and decodes the manifest", func() {
		var gotAgent string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAgent = r.Header.Get("User-Agent")
			_, _ = w.Write([]byte(sampleManifest))
		}))
		defer srv.Close()

		m, err := updater.FetchManifest(context.Background(), srv.Client(), srv.URL+"/version.json")

		Expect(err).NotTo(HaveOccurred())
		Expect(m.LatestVersion).To(Equal("v1.1.0"))
		Expect(gotAgent).To(Equal(updater.UserAgent()))
	})

	It("fails on a non-200 response", func() {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		_, err := updater.FetchManifest(context.Background(), srv.Client(), srv.URL+"/version.json")

		Expect(err).To(MatchError(ContainSubstring("404")))
	})

	It("fails on a manifest without a version", func() {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{}`))
		}))
		defer srv.Close()

		_, err := updater.FetchManifest(context.Background(), srv.Client(), srv.URL)

		Expect(err).To(MatchError(ContainSubstring("latest_version")))
	})

	It("turns a GitHub release into a link-only manifest", func() {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.Header.Get("User-Agent")).To(Equal(updater.UserAgent()))
			_, _ = w.Write([]byte(`{"tag_name":"v1.2.0","body":"notes","html_url":"https://example.test/rel"}`))
		}))
		defer srv.Close()

		m, err := updater.FetchGitHubLatest(context.Background(), srv.Client(), srv.URL)

		Expect(err).NotTo(HaveOccurred())
		Expect(m.LatestVersion).To(Equal("v1.2.0"))
		Expect(m.UpdateMessage).To(Equal("notes"))
		Expect(m.DownloadURL).To(Equal("https://example.test/rel"))
		Expect(m.UpdatePackages).To(BeEmpty())
	})
})
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./pkg/updater/`
Expected: FAIL with `undefined: updater.ManifestURLEnv` and friends.

- [ ] **Step 3: Implement fetch.go**

```go
package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/kpauljoseph/notesankify/pkg/version"
)

const (
	// DefaultManifestURL is where released builds look for updates.
	DefaultManifestURL = "https://notesankify.com/version.json"
	// ManifestURLEnv overrides the manifest URL. It exists to rehearse a
	// release against a locally served manifest before it is published.
	ManifestURLEnv = "NOTESANKIFY_UPDATE_URL"
	// DefaultGitHubLatestURL is the fallback when the manifest is unreachable.
	DefaultGitHubLatestURL = "https://api.github.com/repos/kpauljoseph/notesankify/releases/latest"
)

// ManifestURL returns the manifest location, honouring the override.
func ManifestURL() string {
	if u := strings.TrimSpace(os.Getenv(ManifestURLEnv)); u != "" {
		return u
	}
	return DefaultManifestURL
}

// UserAgent identifies the app, its version and platform to notesankify.com.
// The Cloudflare Worker counts these requests; nothing else is sent.
func UserAgent() string {
	v := strings.TrimPrefix(Normalize(version.Version), "v")
	return fmt.Sprintf("NotesAnkify/%s (%s; %s)", v, runtime.GOOS, runtime.GOARCH)
}

// FetchManifest downloads and decodes version.json.
func FetchManifest(ctx context.Context, client *http.Client, url string) (*VersionResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent())
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch manifest: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manifest: server returned status %d", resp.StatusCode)
	}

	var m VersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	if strings.TrimSpace(m.LatestVersion) == "" {
		return nil, errors.New("manifest has no latest_version")
	}
	return &m, nil
}

// FetchGitHubLatest is the fallback when the manifest is unreachable. The
// result has a download page and no update packages, so the app can only
// offer the link.
func FetchGitHubLatest(ctx context.Context, client *http.Client, url string) (*VersionResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent())
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch GitHub release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var rel GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode GitHub release: %w", err)
	}
	return &VersionResponse{
		LatestVersion: rel.TagName,
		UpdateMessage: rel.Body,
		DownloadURL:   rel.HTMLURL,
	}, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./pkg/updater/`
Expected: PASS. (`checker.go` still exists and still compiles alongside; it is deleted in Task 10.)

- [ ] **Step 5: Commit**

```bash
git add pkg/updater/fetch.go pkg/updater/fetch_test.go
git commit -s -m "feat(updater): fetch the manifest with a descriptive User-Agent and env override"
```

### Task 4: Signature verification

**Files:**
- Create: `pkg/updater/verify.go`
- Create: `pkg/updater/verify_test.go`

**Interfaces:**
- Produces: `func ParsePublicKey(text string) (minisign.PublicKey, error)`, `var ErrBadSignature`, `func VerifyFile(pub minisign.PublicKey, payloadPath, sigPath string) error`

- [ ] **Step 1: Write the failing tests**

`pkg/updater/verify_test.go`:

```go
package updater_test

import (
	"bytes"
	"crypto/rand"
	"io"
	"os"
	"path/filepath"

	"aead.dev/minisign"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kpauljoseph/notesankify/pkg/updater"
)

// hashedSignature signs the way the minisign CLI does by default (prehashed).
func hashedSignature(priv minisign.PrivateKey, payload []byte) []byte {
	r := minisign.NewReader(bytes.NewReader(payload))
	_, err := io.Copy(io.Discard, r)
	Expect(err).NotTo(HaveOccurred())
	return r.Sign(priv)
}

var _ = Describe("Signature verification", func() {
	var (
		pub     minisign.PublicKey
		priv    minisign.PrivateKey
		dir     string
		payload = []byte("update payload bytes")
		payloadPath, sigPath string
	)

	BeforeEach(func() {
		var err error
		pub, priv, err = minisign.GenerateKey(rand.Reader)
		Expect(err).NotTo(HaveOccurred())
		dir = GinkgoT().TempDir()
		payloadPath = filepath.Join(dir, "payload.zip")
		sigPath = payloadPath + ".minisig"
		Expect(os.WriteFile(payloadPath, payload, 0o644)).To(Succeed())
	})

	It("parses a bare base64 key and a full minisign.pub file", func() {
		text, err := pub.MarshalText()
		Expect(err).NotTo(HaveOccurred())

		bare, err := updater.ParsePublicKey(string(text))
		Expect(err).NotTo(HaveOccurred())
		Expect(bare.Equal(pub)).To(BeTrue())

		full, err := updater.ParsePublicKey("untrusted comment: minisign public key\n" + string(text) + "\n")
		Expect(err).NotTo(HaveOccurred())
		Expect(full.Equal(pub)).To(BeTrue())
	})

	It("rejects garbage as a public key", func() {
		_, err := updater.ParsePublicKey("not a key")
		Expect(err).To(HaveOccurred())
	})

	It("accepts a prehashed signature from the release key", func() {
		Expect(os.WriteFile(sigPath, hashedSignature(priv, payload), 0o644)).To(Succeed())
		Expect(updater.VerifyFile(pub, payloadPath, sigPath)).To(Succeed())
	})

	It("accepts a legacy signature from the release key", func() {
		Expect(os.WriteFile(sigPath, minisign.Sign(priv, payload), 0o644)).To(Succeed())
		Expect(updater.VerifyFile(pub, payloadPath, sigPath)).To(Succeed())
	})

	It("rejects a signature from another key", func() {
		_, other, err := minisign.GenerateKey(rand.Reader)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(sigPath, hashedSignature(other, payload), 0o644)).To(Succeed())

		Expect(updater.VerifyFile(pub, payloadPath, sigPath)).To(MatchError(updater.ErrBadSignature))
	})

	It("rejects a tampered payload", func() {
		Expect(os.WriteFile(sigPath, hashedSignature(priv, payload), 0o644)).To(Succeed())
		Expect(os.WriteFile(payloadPath, append(payload, 'x'), 0o644)).To(Succeed())

		Expect(updater.VerifyFile(pub, payloadPath, sigPath)).To(MatchError(updater.ErrBadSignature))
	})

	It("fails when the signature file is missing", func() {
		Expect(updater.VerifyFile(pub, payloadPath, sigPath)).To(MatchError(ContainSubstring("read signature")))
	})
})
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./pkg/updater/`
Expected: FAIL with `undefined: updater.ParsePublicKey`.

- [ ] **Step 3: Implement verify.go**

```go
package updater

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"aead.dev/minisign"
)

// ErrBadSignature means the payload was not signed by the release key. It
// is never retried: the download is discarded and the user gets the link.
var ErrBadSignature = errors.New("update payload signature does not verify")

// ParsePublicKey decodes a minisign public key. text may be the base64 key
// alone or the full contents of a minisign.pub file.
func ParsePublicKey(text string) (minisign.PublicKey, error) {
	var pub minisign.PublicKey
	if err := pub.UnmarshalText([]byte(text)); err != nil {
		return minisign.PublicKey{}, err
	}
	return pub, nil
}

// VerifyFile checks the detached minisign signature at sigPath over the
// whole file at payloadPath. The minisign CLI signs files in prehashed
// mode by default; legacy signatures are accepted too.
func VerifyFile(pub minisign.PublicKey, payloadPath, sigPath string) error {
	sig, err := os.ReadFile(sigPath)
	if err != nil {
		return fmt.Errorf("read signature: %w", err)
	}
	data, err := os.ReadFile(payloadPath)
	if err != nil {
		return fmt.Errorf("read payload: %w", err)
	}

	r := minisign.NewReader(bytes.NewReader(data))
	if _, err := io.Copy(io.Discard, r); err != nil {
		return fmt.Errorf("hash payload: %w", err)
	}
	if r.Verify(pub, sig) {
		return nil
	}
	if minisign.Verify(pub, data, sig) {
		return nil
	}
	return ErrBadSignature
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./pkg/updater/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/updater/verify.go pkg/updater/verify_test.go
git commit -s -m "feat(updater): verify minisign signatures on update payloads"
```

### Task 5: Payload layout check

**Files:**
- Create: `pkg/updater/payload.go`
- Create: `pkg/updater/payload_test.go`
- Create: `pkg/updater/helpers_test.go` (zip builder shared by later tests)

**Interfaces:**
- Produces: `func CheckPayload(goos, payloadPath string) error`, `var ErrUnsupportedPlatform` (defined here, reused by Task 6 and Task 8)
- Test helper: `func writeZip(path string, entries map[string]string)`

- [ ] **Step 1: Write the shared test helper and the failing tests**

`pkg/updater/helpers_test.go`:

```go
package updater_test

import (
	"archive/zip"
	"os"
	"sort"

	. "github.com/onsi/gomega"
)

// writeZip creates a zip at path with the given entries (name -> content).
// Names ending in "/" become directories.
func writeZip(path string, entries map[string]string) {
	f, err := os.Create(path)
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()
	zw := zip.NewWriter(f)
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		hdr := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if name[len(name)-1] == '/' {
			hdr.SetMode(0o755 | os.ModeDir)
		} else if name == "NotesAnkify.app/Contents/MacOS/notesankify-gui" || name == "NotesAnkify.exe" {
			hdr.SetMode(0o755)
		} else {
			hdr.SetMode(0o644)
		}
		w, err := zw.CreateHeader(hdr)
		Expect(err).NotTo(HaveOccurred())
		_, err = w.Write([]byte(entries[name]))
		Expect(err).NotTo(HaveOccurred())
	}
	Expect(zw.Close()).To(Succeed())
}

// macAppEntries is a minimal NotesAnkify.app as the release zip lays it out.
func macAppEntries(binary string) map[string]string {
	return map[string]string{
		"NotesAnkify.app/":                              "",
		"NotesAnkify.app/Contents/":                     "",
		"NotesAnkify.app/Contents/Info.plist":           "<plist/>",
		"NotesAnkify.app/Contents/MacOS/":               "",
		"NotesAnkify.app/Contents/MacOS/notesankify-gui": binary,
		"NotesAnkify.app/Contents/Resources/icon.icns":  "icns",
	}
}
```

`pkg/updater/payload_test.go`:

```go
package updater_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kpauljoseph/notesankify/pkg/updater"
)

var _ = Describe("CheckPayload", func() {
	var dir string

	BeforeEach(func() { dir = GinkgoT().TempDir() })

	It("accepts a macOS bundle zip", func() {
		p := filepath.Join(dir, "mac.zip")
		writeZip(p, macAppEntries("binary"))
		Expect(updater.CheckPayload("darwin", p)).To(Succeed())
	})

	It("rejects a macOS zip without the executable", func() {
		p := filepath.Join(dir, "mac.zip")
		writeZip(p, map[string]string{"NotesAnkify.app/Contents/Info.plist": "<plist/>"})
		Expect(updater.CheckPayload("darwin", p)).To(MatchError(ContainSubstring("notesankify-gui")))
	})

	It("accepts a Windows zip with the exe at the root", func() {
		p := filepath.Join(dir, "win.zip")
		writeZip(p, map[string]string{"NotesAnkify.exe": "MZ"})
		Expect(updater.CheckPayload("windows", p)).To(Succeed())
	})

	It("rejects a Windows zip with the exe in a folder", func() {
		p := filepath.Join(dir, "win.zip")
		writeZip(p, map[string]string{"NotesAnkify/NotesAnkify.exe": "MZ"})
		Expect(updater.CheckPayload("windows", p)).To(MatchError(ContainSubstring("NotesAnkify.exe")))
	})

	It("refuses platforms without in-place updates", func() {
		p := filepath.Join(dir, "x.zip")
		writeZip(p, map[string]string{"a": "b"})
		Expect(updater.CheckPayload("linux", p)).To(MatchError(updater.ErrUnsupportedPlatform))
	})
})
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./pkg/updater/`
Expected: FAIL with `undefined: updater.CheckPayload`.

- [ ] **Step 3: Implement payload.go**

```go
package updater

import (
	"archive/zip"
	"errors"
	"fmt"
)

// ErrUnsupportedPlatform is returned where in-place updates do not exist.
var ErrUnsupportedPlatform = errors.New("in-place updates are not supported on this platform")

// expectedEntries is what a verified payload must contain per OS.
func expectedEntries(goos string) ([]string, error) {
	switch goos {
	case "darwin":
		return []string{
			"NotesAnkify.app/Contents/Info.plist",
			"NotesAnkify.app/Contents/MacOS/notesankify-gui",
		}, nil
	case "windows":
		return []string{"NotesAnkify.exe"}, nil
	default:
		return nil, ErrUnsupportedPlatform
	}
}

// CheckPayload confirms a verified zip has the layout the applier expects.
func CheckPayload(goos, payloadPath string) error {
	want, err := expectedEntries(goos)
	if err != nil {
		return err
	}
	zr, err := zip.OpenReader(payloadPath)
	if err != nil {
		return fmt.Errorf("open update payload: %w", err)
	}
	defer zr.Close()

	have := make(map[string]bool, len(zr.File))
	for _, f := range zr.File {
		have[f.Name] = true
	}
	for _, name := range want {
		if !have[name] {
			return fmt.Errorf("update payload is missing %s", name)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./pkg/updater/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/updater/payload.go pkg/updater/payload_test.go pkg/updater/helpers_test.go
git commit -s -m "feat(updater): check update payload layout before applying"
```

### Task 6: Staging downloads

**Files:**
- Create: `pkg/updater/stage.go`
- Create: `pkg/updater/stage_test.go`

**Interfaces:**
- Consumes: `Package` (Task 2), `UserAgent()` (Task 3)
- Produces: `type Logger interface { Info(string, ...interface{}); Debug(string, ...interface{}) }`, `type NopLogger struct{}`, `const DefaultMaxPayloadBytes = 200 << 20`, `type Staged struct { Version, PayloadPath, SigPath string }`, `type Stager struct { Dir string; Client *http.Client; MaxBytes int64; Log Logger }`, `func DefaultStageDir() (string, error)`, `func NewDownloadClient() *http.Client`, `func (s *Stager) Stage(ctx, version string, pkg Package) (Staged, error)`, `func (s *Stager) Discard(version string) error`, `func (s *Stager) Size(ctx, url string) (int64, error)`

- [ ] **Step 1: Write the failing tests**

`pkg/updater/stage_test.go`:

```go
package updater_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kpauljoseph/notesankify/pkg/updater"
)

var _ = Describe("Stager", func() {
	var (
		srv       *httptest.Server
		hits      atomic.Int32
		payload   = strings.Repeat("p", 1000)
		signature = "untrusted comment: sig\nAAAA\ntrusted comment: t\nBBBB\n"
		stager    *updater.Stager
		pkg       updater.Package
		ctx       = context.Background()
	)

	BeforeEach(func() {
		hits.Store(0)
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			switch r.URL.Path {
			case "/NotesAnkify-macos-universal-v1.1.0.zip":
				w.Header().Set("Content-Length", "1000")
				if r.Method == http.MethodHead {
					return
				}
				_, _ = w.Write([]byte(payload))
			case "/NotesAnkify-macos-universal-v1.1.0.zip.minisig":
				_, _ = w.Write([]byte(signature))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		DeferCleanup(srv.Close)
		stager = &updater.Stager{Dir: GinkgoT().TempDir(), Client: srv.Client(), MaxBytes: 4096, Log: updater.NopLogger{}}
		pkg = updater.Package{
			URL:          srv.URL + "/NotesAnkify-macos-universal-v1.1.0.zip",
			SignatureURL: srv.URL + "/NotesAnkify-macos-universal-v1.1.0.zip.minisig",
		}
	})

	It("downloads the payload and signature into a per-version folder", func() {
		st, err := stager.Stage(ctx, "v1.1.0", pkg)

		Expect(err).NotTo(HaveOccurred())
		Expect(st.Version).To(Equal("v1.1.0"))
		Expect(st.PayloadPath).To(Equal(filepath.Join(stager.Dir, "v1.1.0", "NotesAnkify-macos-universal-v1.1.0.zip")))
		Expect(st.SigPath).To(Equal(st.PayloadPath + ".minisig"))
		Expect(os.ReadFile(st.PayloadPath)).To(Equal([]byte(payload)))
		Expect(os.ReadFile(st.SigPath)).To(Equal([]byte(signature)))
		Expect(filepath.Glob(filepath.Join(stager.Dir, "v1.1.0", "*.part"))).To(BeEmpty())
	})

	It("reuses an already staged payload without downloading again", func() {
		_, err := stager.Stage(ctx, "v1.1.0", pkg)
		Expect(err).NotTo(HaveOccurred())
		before := hits.Load()

		_, err = stager.Stage(ctx, "v1.1.0", pkg)

		Expect(err).NotTo(HaveOccurred())
		Expect(hits.Load()).To(Equal(before))
	})

	It("removes staged folders for other versions", func() {
		old := filepath.Join(stager.Dir, "v1.0.5")
		Expect(os.MkdirAll(old, 0o755)).To(Succeed())

		_, err := stager.Stage(ctx, "v1.1.0", pkg)

		Expect(err).NotTo(HaveOccurred())
		Expect(old).NotTo(BeADirectory())
	})

	It("refuses payloads over the size cap and leaves no partial file", func() {
		stager.MaxBytes = 100

		_, err := stager.Stage(ctx, "v1.1.0", pkg)

		Expect(err).To(MatchError(ContainSubstring("exceeds")))
		Expect(filepath.Glob(filepath.Join(stager.Dir, "v1.1.0", "*"))).To(ConsistOf(HaveSuffix(".minisig")))
	})

	It("fails when the server does not have the payload", func() {
		pkg.URL = srv.URL + "/missing.zip"

		_, err := stager.Stage(ctx, "v1.1.0", pkg)

		Expect(err).To(MatchError(ContainSubstring("404")))
	})

	It("reports the payload size from a HEAD request", func() {
		Expect(stager.Size(ctx, pkg.URL)).To(BeEquivalentTo(1000))
	})

	It("discards a staged version", func() {
		_, err := stager.Stage(ctx, "v1.1.0", pkg)
		Expect(err).NotTo(HaveOccurred())

		Expect(stager.Discard("v1.1.0")).To(Succeed())

		Expect(filepath.Join(stager.Dir, "v1.1.0")).NotTo(BeADirectory())
	})
})
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./pkg/updater/`
Expected: FAIL with `undefined: updater.Stager`.

- [ ] **Step 3: Implement stage.go**

```go
package updater

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"
)

// DefaultMaxPayloadBytes caps a single update download.
const DefaultMaxPayloadBytes = 200 << 20

// idleTimeout aborts a download that makes no progress for this long.
const idleTimeout = 60 * time.Second

// Logger is the subset of pkg/logger the updater uses.
type Logger interface {
	Info(format string, args ...interface{})
	Debug(format string, args ...interface{})
}

// NopLogger discards everything. Tests use it.
type NopLogger struct{}

func (NopLogger) Info(string, ...interface{})  {}
func (NopLogger) Debug(string, ...interface{}) {}

// Staged is a downloaded payload and its signature on disk.
type Staged struct {
	Version     string
	PayloadPath string
	SigPath     string
}

// Stager downloads update payloads into Dir/<version>/.
type Stager struct {
	Dir      string
	Client   *http.Client
	MaxBytes int64
	Log      Logger
}

// DefaultStageDir returns <user cache dir>/NotesAnkify/updates.
func DefaultStageDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "NotesAnkify", "updates"), nil
}

// NewDownloadClient returns a client suited to multi-megabyte downloads:
// bounded time to connect and to receive headers, no overall deadline.
// Progress is guarded per read by the idle timeout in download.
func NewDownloadClient() *http.Client {
	return &http.Client{Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 15 * time.Second}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}}
}

// Stage downloads the payload and signature for version unless both are
// already present. Staged folders for other versions are removed first.
func (s *Stager) Stage(ctx context.Context, version string, pkg Package) (Staged, error) {
	if err := s.cleanupExcept(version); err != nil {
		s.log().Debug("Could not clean old staged updates: %v", err)
	}
	dir := filepath.Join(s.Dir, version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Staged{}, fmt.Errorf("create staging folder: %w", err)
	}
	name, err := fileNameFromURL(pkg.URL)
	if err != nil {
		return Staged{}, err
	}
	st := Staged{
		Version:     version,
		PayloadPath: filepath.Join(dir, name),
		SigPath:     filepath.Join(dir, name+".minisig"),
	}
	if isRegularFile(st.PayloadPath) && isRegularFile(st.SigPath) {
		s.log().Debug("Reusing staged update %s", st.PayloadPath)
		return st, nil
	}
	if err := s.download(ctx, pkg.SignatureURL, st.SigPath, 64<<10); err != nil {
		return Staged{}, fmt.Errorf("download signature: %w", err)
	}
	if err := s.download(ctx, pkg.URL, st.PayloadPath, s.maxBytes()); err != nil {
		return Staged{}, fmt.Errorf("download payload: %w", err)
	}
	return st, nil
}

// Discard removes everything staged for version.
func (s *Stager) Discard(version string) error {
	return os.RemoveAll(filepath.Join(s.Dir, version))
}

// Size asks the server for the payload size without downloading it.
func (s *Stager) Size(ctx context.Context, rawURL string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", UserAgent())
	resp, err := s.Client.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HEAD %s: status %d", rawURL, resp.StatusCode)
	}
	if resp.ContentLength < 0 {
		return 0, errors.New("server did not report the payload size")
	}
	return resp.ContentLength, nil
}

func (s *Stager) maxBytes() int64 {
	if s.MaxBytes > 0 {
		return s.MaxBytes
	}
	return DefaultMaxPayloadBytes
}

func (s *Stager) log() Logger {
	if s.Log == nil {
		return NopLogger{}
	}
	return s.Log
}

// download fetches rawURL into dst via a temporary ".part" file, so a
// partial download is never mistaken for a complete one.
func (s *Stager) download(ctx context.Context, rawURL, dst string, max int64) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent())
	resp, err := s.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", rawURL, resp.StatusCode)
	}

	tmp := dst + ".part"
	defer os.Remove(tmp)
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	body := &idleReader{r: resp.Body, timer: time.AfterFunc(idleTimeout, cancel)}
	defer body.timer.Stop()
	n, err := io.Copy(f, io.LimitReader(body, max+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if n > max {
		return fmt.Errorf("download of %s exceeds the %d byte limit", rawURL, max)
	}
	return os.Rename(tmp, dst)
}

// idleReader resets a timer on every read; the timer cancels the request
// context when no bytes arrive for idleTimeout.
type idleReader struct {
	r     io.Reader
	timer *time.Timer
}

func (ir *idleReader) Read(p []byte) (int, error) {
	n, err := ir.r.Read(p)
	ir.timer.Reset(idleTimeout)
	return n, err
}

func (s *Stager) cleanupExcept(keep string) error {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() && e.Name() != keep {
			if err := os.RemoveAll(filepath.Join(s.Dir, e.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

func fileNameFromURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	name := path.Base(u.Path)
	if name == "" || name == "." || name == "/" {
		return "", fmt.Errorf("cannot derive a file name from %s", raw)
	}
	return name, nil
}

func isRegularFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./pkg/updater/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/updater/stage.go pkg/updater/stage_test.go
git commit -s -m "feat(updater): stage signed payload downloads per version"
```

### Task 7: Install target and in-place checks

**Files:**
- Create: `pkg/updater/inplace.go`
- Create: `pkg/updater/free_unix.go`, `pkg/updater/free_windows.go`, `pkg/updater/free_other.go`
- Create: `pkg/updater/inplace_test.go`

**Interfaces:**
- Consumes: `ErrUnsupportedPlatform` (Task 5)
- Produces: `type TargetKind int` with `KindBundle`, `KindExe`; `type InstallTarget struct { Kind TargetKind; Path string }`; `type Probe interface { Executable() (string, error); Writable(dir string) bool; FreeBytes(dir string) (uint64, error) }`; `type OSProbe struct{}`; `func FindTarget(goos string, probe Probe) (InstallTarget, error)`; `const MoveToApplicationsHint`; `func CheckInPlace(goos string, target InstallTarget, probe Probe, stageDir string, payloadSize int64) (reason string, ok bool)`

- [ ] **Step 1: Write the failing tests**

`pkg/updater/inplace_test.go`:

```go
package updater_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kpauljoseph/notesankify/pkg/updater"
)

// fakeProbe answers the in-place checks from fixed values.
type fakeProbe struct {
	exe      string
	exeErr   error
	writable bool
	free     uint64
	freeErr  error
}

func (f fakeProbe) Executable() (string, error)           { return f.exe, f.exeErr }
func (f fakeProbe) Writable(string) bool                   { return f.writable }
func (f fakeProbe) FreeBytes(string) (uint64, error)       { return f.free, f.freeErr }

const macExe = "/Applications/NotesAnkify.app/Contents/MacOS/notesankify-gui"

var _ = Describe("FindTarget", func() {
	It("derives the bundle from the macOS executable path", func() {
		t, err := updater.FindTarget("darwin", fakeProbe{exe: macExe})
		Expect(err).NotTo(HaveOccurred())
		Expect(t.Kind).To(Equal(updater.KindBundle))
		Expect(t.Path).To(Equal("/Applications/NotesAnkify.app"))
	})

	It("refuses a macOS executable that is not inside a bundle", func() {
		_, err := updater.FindTarget("darwin", fakeProbe{exe: "/usr/local/bin/notesankify-gui"})
		Expect(err).To(MatchError(ContainSubstring("not running from an app bundle")))
	})

	It("uses the executable itself on Windows", func() {
		t, err := updater.FindTarget("windows", fakeProbe{exe: `C:\Users\me\NotesAnkify\NotesAnkify.exe`})
		Expect(err).NotTo(HaveOccurred())
		Expect(t.Kind).To(Equal(updater.KindExe))
		Expect(t.Path).To(Equal(`C:\Users\me\NotesAnkify\NotesAnkify.exe`))
	})

	It("reports other platforms as unsupported", func() {
		_, err := updater.FindTarget("linux", fakeProbe{exe: "/usr/local/bin/notesankify-gui"})
		Expect(err).To(MatchError(updater.ErrUnsupportedPlatform))
	})

	It("propagates executable lookup errors", func() {
		_, err := updater.FindTarget("darwin", fakeProbe{exeErr: errors.New("boom")})
		Expect(err).To(MatchError("boom"))
	})
})

var _ = Describe("CheckInPlace", func() {
	bundle := updater.InstallTarget{Kind: updater.KindBundle, Path: "/Applications/NotesAnkify.app"}
	const size = int64(40 << 20)

	It("allows a writable install with enough space", func() {
		reason, ok := updater.CheckInPlace("darwin", bundle, fakeProbe{writable: true, free: 1 << 30}, "/tmp/stage", size)
		Expect(ok).To(BeTrue())
		Expect(reason).To(BeEmpty())
	})

	It("refuses an app running from a mounted DMG", func() {
		t := updater.InstallTarget{Kind: updater.KindBundle, Path: "/Volumes/NotesAnkify/NotesAnkify.app"}
		reason, ok := updater.CheckInPlace("darwin", t, fakeProbe{writable: true, free: 1 << 30}, "/tmp/stage", size)
		Expect(ok).To(BeFalse())
		Expect(reason).To(Equal(updater.MoveToApplicationsHint))
	})

	It("refuses a translocated app", func() {
		t := updater.InstallTarget{Kind: updater.KindBundle, Path: "/private/var/folders/xx/T/AppTranslocation/1234/d/NotesAnkify.app"}
		reason, ok := updater.CheckInPlace("darwin", t, fakeProbe{writable: true, free: 1 << 30}, "/tmp/stage", size)
		Expect(ok).To(BeFalse())
		Expect(reason).To(Equal(updater.MoveToApplicationsHint))
	})

	It("refuses when the install folder is not writable", func() {
		reason, ok := updater.CheckInPlace("darwin", bundle, fakeProbe{writable: false, free: 1 << 30}, "/tmp/stage", size)
		Expect(ok).To(BeFalse())
		Expect(reason).To(ContainSubstring("cannot write to /Applications"))
	})

	It("refuses when free space is under three times the payload", func() {
		reason, ok := updater.CheckInPlace("darwin", bundle, fakeProbe{writable: true, free: 100 << 20}, "/tmp/stage", size)
		Expect(ok).To(BeFalse())
		Expect(reason).To(ContainSubstring("free space"))
	})

	It("does not block when free space cannot be determined", func() {
		_, ok := updater.CheckInPlace("darwin", bundle, fakeProbe{writable: true, freeErr: errors.New("statfs")}, "/tmp/stage", size)
		Expect(ok).To(BeTrue())
	})
})
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./pkg/updater/`
Expected: FAIL with `undefined: updater.FindTarget`.

- [ ] **Step 3: Implement inplace.go and the free-space files**

`pkg/updater/inplace.go`:

```go
package updater

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TargetKind says what the updater replaces.
type TargetKind int

const (
	// KindBundle is a macOS .app directory.
	KindBundle TargetKind = iota + 1
	// KindExe is a single executable file, the Windows layout.
	KindExe
)

// InstallTarget is the installed thing an update replaces.
type InstallTarget struct {
	Kind TargetKind
	Path string // bundle directory or executable path
}

// Probe answers questions about the local machine. Tests substitute a fake.
type Probe interface {
	Executable() (string, error)
	Writable(dir string) bool
	FreeBytes(dir string) (uint64, error)
}

// OSProbe is the real Probe.
type OSProbe struct{}

// Executable returns the running program's path with symlinks resolved.
func (OSProbe) Executable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// Writable reports whether a file can be created in dir.
func (OSProbe) Writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".notesankify-write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}

// MoveToApplicationsHint is shown when the app runs from a DMG or from
// macOS's translocation folder, where it cannot replace itself.
const MoveToApplicationsHint = "Move NotesAnkify to your Applications folder to enable automatic updates."

// FindTarget derives the install target from the running executable.
func FindTarget(goos string, probe Probe) (InstallTarget, error) {
	exe, err := probe.Executable()
	if err != nil {
		return InstallTarget{}, err
	}
	switch goos {
	case "darwin":
		// <bundle>.app/Contents/MacOS/<executable>
		macos := filepath.Dir(exe)
		contents := filepath.Dir(macos)
		bundle := filepath.Dir(contents)
		if filepath.Base(macos) != "MacOS" || filepath.Base(contents) != "Contents" || !strings.HasSuffix(bundle, ".app") {
			return InstallTarget{}, fmt.Errorf("not running from an app bundle: %s", exe)
		}
		return InstallTarget{Kind: KindBundle, Path: bundle}, nil
	case "windows":
		return InstallTarget{Kind: KindExe, Path: exe}, nil
	default:
		return InstallTarget{}, ErrUnsupportedPlatform
	}
}

// CheckInPlace decides whether target can be replaced in place. On refusal
// it returns a one-line reason suitable for the user.
func CheckInPlace(goos string, target InstallTarget, probe Probe, stageDir string, payloadSize int64) (reason string, ok bool) {
	if goos == "darwin" {
		if strings.HasPrefix(target.Path, "/Volumes/") || strings.Contains(target.Path, "/AppTranslocation/") {
			return MoveToApplicationsHint, false
		}
	}
	parent := filepath.Dir(target.Path)
	if !probe.Writable(parent) {
		return fmt.Sprintf("NotesAnkify cannot write to %s, so it cannot update itself.", parent), false
	}
	need := uint64(payloadSize) * 3
	for _, dir := range []string{parent, stageDir} {
		free, err := probe.FreeBytes(dir)
		if err != nil {
			continue // unknown is not a reason to refuse
		}
		if free < need {
			return fmt.Sprintf("Not enough free space in %s to update (%d MB needed).", dir, need>>20), false
		}
	}
	return "", true
}
```

`pkg/updater/free_unix.go`:

```go
//go:build darwin || linux

package updater

import "syscall"

// FreeBytes returns the bytes available to this user on dir's volume.
func (OSProbe) FreeBytes(dir string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
```

`pkg/updater/free_windows.go`:

```go
//go:build windows

package updater

import "golang.org/x/sys/windows"

// FreeBytes returns the bytes available to this user on dir's volume.
func (OSProbe) FreeBytes(dir string) (uint64, error) {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var free, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(p, &free, &total, &totalFree); err != nil {
		return 0, err
	}
	return free, nil
}
```

`pkg/updater/free_other.go`:

```go
//go:build !darwin && !linux && !windows

package updater

import "errors"

// FreeBytes is unknown on platforms without a probe; CheckInPlace treats
// an error as "do not block".
func (OSProbe) FreeBytes(string) (uint64, error) {
	return 0, errors.New("free space unknown on this platform")
}
```

- [ ] **Step 4: Run the tests and vet on all three OSes' build tags**

Run:
```bash
go test ./pkg/updater/
GOOS=windows go vet ./pkg/updater/
GOOS=darwin go vet ./pkg/updater/
```
Expected: PASS, and both vets clean (the package has no cgo, so cross-vet works).

- [ ] **Step 5: Commit**

```bash
git add pkg/updater/inplace.go pkg/updater/free_unix.go pkg/updater/free_windows.go pkg/updater/free_other.go pkg/updater/inplace_test.go go.mod go.sum
git commit -s -m "feat(updater): find the install target and check it can be replaced in place"
```

### Task 8: Extraction and bundle swap

**Files:**
- Create: `pkg/updater/extract.go`, `pkg/updater/extract_darwin.go`, `pkg/updater/extract_other.go`
- Create: `pkg/updater/bundleswap.go`
- Create: `pkg/updater/extract_test.go`, `pkg/updater/bundleswap_test.go`

**Interfaces:**
- Produces: `type Extractor interface { Extract(zipPath, destDir string) error }`, `type ZipExtractor struct{}`, `type DittoExtractor struct{}` (darwin only), `func DefaultExtractor() Extractor`, `func SwapBundle(current, staged string) (backup string, err error)`

- [ ] **Step 1: Write the failing tests**

`pkg/updater/extract_test.go`:

```go
package updater_test

import (
	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kpauljoseph/notesankify/pkg/updater"
)

var _ = Describe("ZipExtractor", func() {
	var dir string

	BeforeEach(func() { dir = GinkgoT().TempDir() })

	It("recreates directories, files and the executable bit", func() {
		zipPath := filepath.Join(dir, "mac.zip")
		writeZip(zipPath, macAppEntries("binary"))
		dest := filepath.Join(dir, "out")

		Expect(updater.ZipExtractor{}.Extract(zipPath, dest)).To(Succeed())

		bin := filepath.Join(dest, "NotesAnkify.app", "Contents", "MacOS", "notesankify-gui")
		Expect(os.ReadFile(bin)).To(Equal([]byte("binary")))
		Expect(os.ReadFile(filepath.Join(dest, "NotesAnkify.app", "Contents", "Info.plist"))).To(Equal([]byte("<plist/>")))
		if runtime.GOOS != "windows" {
			info, err := os.Stat(bin)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode().Perm() & 0o100).NotTo(BeZero(), "executable bit kept")
		}
	})

	It("refuses entries that escape the destination", func() {
		zipPath := filepath.Join(dir, "evil.zip")
		writeZip(zipPath, map[string]string{"../escape.txt": "x"})

		Expect(updater.ZipExtractor{}.Extract(zipPath, filepath.Join(dir, "out"))).To(MatchError(ContainSubstring("unsafe path")))
	})
})
```

`pkg/updater/bundleswap_test.go`:

```go
package updater_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kpauljoseph/notesankify/pkg/updater"
)

func writeMarker(bundle, content string) {
	Expect(os.MkdirAll(filepath.Join(bundle, "Contents"), 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(bundle, "Contents", "marker"), []byte(content), 0o644)).To(Succeed())
}

func readMarker(bundle string) string {
	b, err := os.ReadFile(filepath.Join(bundle, "Contents", "marker"))
	Expect(err).NotTo(HaveOccurred())
	return string(b)
}

var _ = Describe("SwapBundle", func() {
	var dir, current, staged string

	BeforeEach(func() {
		dir = GinkgoT().TempDir()
		current = filepath.Join(dir, "NotesAnkify.app")
		staged = filepath.Join(dir, ".tmp", "NotesAnkify.app")
		writeMarker(current, "old")
		writeMarker(staged, "new")
	})

	It("puts the staged bundle in place and keeps the old one aside", func() {
		backup, err := updater.SwapBundle(current, staged)

		Expect(err).NotTo(HaveOccurred())
		Expect(backup).To(Equal(current + ".old"))
		Expect(readMarker(current)).To(Equal("new"))
		Expect(readMarker(backup)).To(Equal("old"))
		Expect(staged).NotTo(BeADirectory())
	})

	It("replaces a stale backup from an earlier update", func() {
		writeMarker(current+".old", "stale")

		_, err := updater.SwapBundle(current, staged)

		Expect(err).NotTo(HaveOccurred())
		Expect(readMarker(current + ".old")).To(Equal("old"))
	})

	It("restores the current bundle when the staged one cannot be moved", func() {
		Expect(os.RemoveAll(staged)).To(Succeed())

		_, err := updater.SwapBundle(current, staged)

		Expect(err).To(HaveOccurred())
		Expect(readMarker(current)).To(Equal("old"))
		Expect(current + ".old").NotTo(BeADirectory())
	})
})
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./pkg/updater/`
Expected: FAIL with `undefined: updater.ZipExtractor` and `undefined: updater.SwapBundle`.

- [ ] **Step 3: Implement extract.go, the platform files and bundleswap.go**

`pkg/updater/extract.go`:

```go
package updater

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Extractor expands a zip payload into destDir.
type Extractor interface {
	Extract(zipPath, destDir string) error
}

// ZipExtractor uses Go's archive/zip. It is the extractor on Windows and
// in tests. On macOS DittoExtractor is preferred because ditto restores
// bundle metadata exactly.
type ZipExtractor struct{}

// Extract writes every entry under destDir, refusing paths that escape it.
func (ZipExtractor) Extract(zipPath, destDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		// Go 1.20+ flags names with ".." or a leading "/" while still
		// returning a usable reader; refuse the whole archive instead.
		if zr != nil {
			zr.Close()
		}
		if errors.Is(err, zip.ErrInsecurePath) {
			return fmt.Errorf("zip contains an entry with an unsafe path: %w", err)
		}
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "/") || strings.Contains(f.Name, "..") {
			return fmt.Errorf("zip entry %q has an unsafe path", f.Name)
		}
		if f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("zip entry %q is a symlink, which update payloads never contain", f.Name)
		}
		dst := filepath.Join(destDir, filepath.FromSlash(f.Name))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := extractFile(f, dst); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(f *zip.File, dst string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	mode := f.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
```

`pkg/updater/extract_darwin.go`:

```go
//go:build darwin

package updater

import (
	"fmt"
	"os/exec"
	"strings"
)

// DittoExtractor shells out to /usr/bin/ditto, which restores bundle
// metadata, extended attributes and permissions exactly as zipped.
type DittoExtractor struct{}

// Extract runs ditto -x -k.
func (DittoExtractor) Extract(zipPath, destDir string) error {
	out, err := exec.Command("/usr/bin/ditto", "-x", "-k", zipPath, destDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ditto: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// DefaultExtractor is ditto on macOS.
func DefaultExtractor() Extractor { return DittoExtractor{} }
```

`pkg/updater/extract_other.go`:

```go
//go:build !darwin

package updater

// DefaultExtractor is the pure-Go zip reader everywhere but macOS.
func DefaultExtractor() Extractor { return ZipExtractor{} }
```

`pkg/updater/bundleswap.go`:

```go
package updater

import (
	"fmt"
	"os"
)

// SwapBundle replaces the installed bundle at current with the one at
// staged using two renames in the same directory. On failure it restores
// current and returns the error. It returns the path the previous bundle
// was moved to; the new build deletes it on its first successful start.
func SwapBundle(current, staged string) (backup string, err error) {
	backup = current + ".old"
	if err := os.RemoveAll(backup); err != nil {
		return "", fmt.Errorf("remove stale backup: %w", err)
	}
	if err := os.Rename(current, backup); err != nil {
		return "", fmt.Errorf("move current bundle aside: %w", err)
	}
	if err := os.Rename(staged, current); err != nil {
		if rb := os.Rename(backup, current); rb != nil {
			return "", fmt.Errorf("install new bundle: %v; rollback failed: %w", err, rb)
		}
		return "", fmt.Errorf("install new bundle: %w", err)
	}
	return backup, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./pkg/updater/ && GOOS=darwin go vet ./pkg/updater/`
Expected: PASS and clean vet.

- [ ] **Step 5: Commit**

```bash
git add pkg/updater/extract.go pkg/updater/extract_darwin.go pkg/updater/extract_other.go pkg/updater/bundleswap.go pkg/updater/extract_test.go pkg/updater/bundleswap_test.go
git commit -s -m "feat(updater): extract payloads and swap the macOS bundle atomically"
```

### Task 9: Apply, relaunch and leftover cleanup

**Files:**
- Create: `pkg/updater/apply.go`
- Create: `pkg/updater/relaunch.go`, `pkg/updater/relaunch_darwin.go`, `pkg/updater/relaunch_windows.go`, `pkg/updater/relaunch_other.go`
- Create: `pkg/updater/cleanup.go`
- Create: `pkg/updater/apply_test.go`, `pkg/updater/cleanup_test.go`

**Interfaces:**
- Consumes: `InstallTarget`, `KindBundle`, `KindExe`, `Probe`, `FindTarget` (Task 7); `Extractor`, `SwapBundle` (Task 8); `Logger` (Task 6)
- Produces: `type Applier struct { GOOS string; Extractor Extractor; Log Logger }`, `func (a *Applier) Apply(target InstallTarget, payloadPath string) error`, `var ErrRelaunch`, `func Relaunch(target InstallTarget) error`, `func RemoveLeftovers(goos string, probe Probe) error`

- [ ] **Step 1: Write the failing tests**

`pkg/updater/apply_test.go`:

```go
package updater_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kpauljoseph/notesankify/pkg/updater"
)

var _ = Describe("Applier", func() {
	var dir string

	BeforeEach(func() { dir = GinkgoT().TempDir() })

	Describe("on a macOS bundle", func() {
		var installed, payload string

		BeforeEach(func() {
			installed = filepath.Join(dir, "Applications", "NotesAnkify.app")
			Expect(os.MkdirAll(filepath.Join(installed, "Contents", "MacOS"), 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(installed, "Contents", "MacOS", "notesankify-gui"), []byte("old"), 0o755)).To(Succeed())
			payload = filepath.Join(dir, "mac.zip")
			writeZip(payload, macAppEntries("new"))
		})

		It("installs the new bundle and keeps the old one as .old", func() {
			a := &updater.Applier{GOOS: "darwin", Extractor: updater.ZipExtractor{}, Log: updater.NopLogger{}}

			Expect(a.Apply(updater.InstallTarget{Kind: updater.KindBundle, Path: installed}, payload)).To(Succeed())

			Expect(os.ReadFile(filepath.Join(installed, "Contents", "MacOS", "notesankify-gui"))).To(Equal([]byte("new")))
			Expect(os.ReadFile(filepath.Join(installed+".old", "Contents", "MacOS", "notesankify-gui"))).To(Equal([]byte("old")))
			Expect(filepath.Glob(filepath.Join(dir, "Applications", ".notesankify-update-*"))).To(BeEmpty(), "temporary folder removed")
		})

		It("leaves the installed bundle alone when the payload has no bundle", func() {
			writeZip(payload, map[string]string{"README.txt": "x"})
			a := &updater.Applier{GOOS: "darwin", Extractor: updater.ZipExtractor{}, Log: updater.NopLogger{}}

			Expect(a.Apply(updater.InstallTarget{Kind: updater.KindBundle, Path: installed}, payload)).To(MatchError(ContainSubstring("no .app bundle")))

			Expect(os.ReadFile(filepath.Join(installed, "Contents", "MacOS", "notesankify-gui"))).To(Equal([]byte("old")))
		})
	})

	Describe("on a Windows executable", func() {
		It("replaces the executable's contents", func() {
			exe := filepath.Join(dir, "NotesAnkify.exe")
			Expect(os.WriteFile(exe, []byte("old"), 0o755)).To(Succeed())
			payload := filepath.Join(dir, "win.zip")
			writeZip(payload, map[string]string{"NotesAnkify.exe": "new"})
			a := &updater.Applier{GOOS: "windows", Extractor: updater.ZipExtractor{}, Log: updater.NopLogger{}}

			Expect(a.Apply(updater.InstallTarget{Kind: updater.KindExe, Path: exe}, payload)).To(Succeed())

			Expect(os.ReadFile(exe)).To(Equal([]byte("new")))
		})

		It("fails when the zip has no executable of that name", func() {
			exe := filepath.Join(dir, "NotesAnkify.exe")
			Expect(os.WriteFile(exe, []byte("old"), 0o755)).To(Succeed())
			payload := filepath.Join(dir, "win.zip")
			writeZip(payload, map[string]string{"Other.exe": "new"})
			a := &updater.Applier{GOOS: "windows", Extractor: updater.ZipExtractor{}, Log: updater.NopLogger{}}

			Expect(a.Apply(updater.InstallTarget{Kind: updater.KindExe, Path: exe}, payload)).To(MatchError(ContainSubstring("does not contain NotesAnkify.exe")))
			Expect(os.ReadFile(exe)).To(Equal([]byte("old")))
		})
	})
})
```

`pkg/updater/cleanup_test.go`:

```go
package updater_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kpauljoseph/notesankify/pkg/updater"
)

var _ = Describe("RemoveLeftovers", func() {
	var dir string

	BeforeEach(func() { dir = GinkgoT().TempDir() })

	It("removes the previous macOS bundle after an update", func() {
		bundle := filepath.Join(dir, "NotesAnkify.app")
		exe := filepath.Join(bundle, "Contents", "MacOS", "notesankify-gui")
		Expect(os.MkdirAll(filepath.Dir(exe), 0o755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(bundle+".old", "Contents"), 0o755)).To(Succeed())

		Expect(updater.RemoveLeftovers("darwin", fakeProbe{exe: exe})).To(Succeed())

		Expect(bundle + ".old").NotTo(BeADirectory())
		Expect(bundle).To(BeADirectory())
	})

	It("removes the hidden old Windows executable", func() {
		exe := filepath.Join(dir, "NotesAnkify.exe")
		old := filepath.Join(dir, ".NotesAnkify.exe.old")
		Expect(os.WriteFile(exe, []byte("new"), 0o755)).To(Succeed())
		Expect(os.WriteFile(old, []byte("old"), 0o755)).To(Succeed())

		Expect(updater.RemoveLeftovers("windows", fakeProbe{exe: exe})).To(Succeed())

		Expect(old).NotTo(BeAnExistingFile())
	})

	It("does nothing when there is nothing to remove or the layout is unknown", func() {
		Expect(updater.RemoveLeftovers("darwin", fakeProbe{exe: "/usr/local/bin/notesankify-gui"})).To(Succeed())
		Expect(updater.RemoveLeftovers("linux", fakeProbe{exe: "/usr/local/bin/notesankify-gui"})).To(Succeed())
	})
})
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./pkg/updater/`
Expected: FAIL with `undefined: updater.Applier` and `undefined: updater.RemoveLeftovers`.

- [ ] **Step 3: Implement apply.go**

```go
package updater

import (
	"archive/zip"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/minio/selfupdate"
)

// Applier installs a verified payload over the running program. It does
// not relaunch; see Relaunch.
type Applier struct {
	GOOS      string
	Extractor Extractor
	Log       Logger
}

// Apply installs payloadPath onto target.
func (a *Applier) Apply(target InstallTarget, payloadPath string) error {
	switch target.Kind {
	case KindBundle:
		return a.applyBundle(target, payloadPath)
	case KindExe:
		return a.applyExe(target, payloadPath)
	default:
		return fmt.Errorf("unknown install target kind %d", target.Kind)
	}
}

// applyBundle extracts next to the installed app, so the final renames
// stay on one volume, then swaps the bundles.
func (a *Applier) applyBundle(target InstallTarget, payloadPath string) error {
	parent := filepath.Dir(target.Path)
	tmp, err := os.MkdirTemp(parent, ".notesankify-update-")
	if err != nil {
		return fmt.Errorf("create staging folder next to the app: %w", err)
	}
	defer os.RemoveAll(tmp)

	if err := a.Extractor.Extract(payloadPath, tmp); err != nil {
		return fmt.Errorf("extract update: %w", err)
	}
	staged, err := findBundle(tmp)
	if err != nil {
		return err
	}
	if err := checkBundleLayout(staged); err != nil {
		return err
	}
	backup, err := SwapBundle(target.Path, staged)
	if err != nil {
		return err
	}
	a.log().Info("Installed update; previous version kept at %s until the next start", backup)
	return nil
}

// findBundle returns the single .app directory inside dir.
func findBundle(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() && strings.HasSuffix(e.Name(), ".app") {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", errors.New("update payload contains no .app bundle")
}

// checkBundleLayout confirms the extracted bundle is an app bundle.
func checkBundleLayout(bundle string) error {
	for _, rel := range []string{"Contents/Info.plist", "Contents/MacOS"} {
		if _, err := os.Stat(filepath.Join(bundle, filepath.FromSlash(rel))); err != nil {
			return fmt.Errorf("update payload is not an app bundle (missing %s)", rel)
		}
	}
	return nil
}

// applyExe streams the executable out of the zip into minio/selfupdate,
// which renames the running file aside, writes the new one, and rolls
// back if anything fails.
func (a *Applier) applyExe(target InstallTarget, payloadPath string) error {
	zr, err := zip.OpenReader(payloadPath)
	if err != nil {
		return fmt.Errorf("open update payload: %w", err)
	}
	defer zr.Close()

	want := filepath.Base(target.Path)
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || path.Base(f.Name) != want {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		if err := selfupdate.Apply(rc, selfupdate.Options{TargetPath: target.Path}); err != nil {
			if rerr := selfupdate.RollbackError(err); rerr != nil {
				return fmt.Errorf("update failed and rollback failed, reinstall NotesAnkify: %v", rerr)
			}
			return fmt.Errorf("install update: %w", err)
		}
		return nil
	}
	return fmt.Errorf("update payload does not contain %s", want)
}

func (a *Applier) log() Logger {
	if a.Log == nil {
		return NopLogger{}
	}
	return a.Log
}
```

- [ ] **Step 4: Implement the relaunch files and cleanup.go**

`pkg/updater/relaunch.go`:

```go
package updater

import "errors"

// ErrRelaunch means the update was installed but the new build could not
// be started. The user starts NotesAnkify again by hand; nothing is lost.
var ErrRelaunch = errors.New("update installed but NotesAnkify could not restart itself; please start it again")
```

`pkg/updater/relaunch_darwin.go`:

```go
//go:build darwin

package updater

import "os/exec"

// Relaunch starts a new instance of the bundle via LaunchServices.
func Relaunch(target InstallTarget) error {
	return exec.Command("/usr/bin/open", "-n", target.Path).Run()
}
```

`pkg/updater/relaunch_windows.go`:

```go
//go:build windows

package updater

import (
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

// Relaunch starts the new executable detached from this process, so it
// survives this process exiting.
func Relaunch(target InstallTarget) error {
	cmd := exec.Command(target.Path)
	cmd.Dir = filepath.Dir(target.Path)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}
	return cmd.Start()
}
```

`pkg/updater/relaunch_other.go`:

```go
//go:build !darwin && !windows

package updater

// Relaunch is not available where in-place updates do not exist.
func Relaunch(InstallTarget) error { return ErrUnsupportedPlatform }
```

`pkg/updater/cleanup.go`:

```go
package updater

import (
	"os"
	"path/filepath"
	"strings"
)

// RemoveLeftovers deletes what the previous build left behind after an
// update: NotesAnkify.app.old on macOS, the hidden .NotesAnkify.exe.old on
// Windows. It is safe to call on every start and never removes anything
// the running program lives in.
func RemoveLeftovers(goos string, probe Probe) error {
	target, err := FindTarget(goos, probe)
	if err != nil {
		return nil // not an installed layout; nothing to clean
	}
	var leftover string
	switch target.Kind {
	case KindBundle:
		leftover = target.Path + ".old"
	case KindExe:
		leftover = filepath.Join(filepath.Dir(target.Path), "."+filepath.Base(target.Path)+".old")
	default:
		return nil
	}
	if target.Path == leftover || strings.HasPrefix(target.Path, leftover+string(os.PathSeparator)) {
		return nil
	}
	if _, err := os.Stat(leftover); err != nil {
		return nil
	}
	return os.RemoveAll(leftover)
}
```

- [ ] **Step 5: Run the tests and cross-vet**

Run:
```bash
go test ./pkg/updater/
GOOS=windows go vet ./pkg/updater/
GOOS=darwin go vet ./pkg/updater/
```
Expected: PASS, clean vets.

- [ ] **Step 6: Commit**

```bash
git add pkg/updater/apply.go pkg/updater/relaunch.go pkg/updater/relaunch_darwin.go pkg/updater/relaunch_windows.go pkg/updater/relaunch_other.go pkg/updater/cleanup.go pkg/updater/apply_test.go pkg/updater/cleanup_test.go go.mod go.sum
git commit -s -m "feat(updater): apply payloads in place, relaunch, and clean up leftovers"
```

### Task 10: The update manager

**Files:**
- Create: `pkg/updater/manager.go`
- Create: `pkg/updater/manager_test.go`

**Interfaces:**
- Consumes: everything from Tasks 1 to 9
- Produces: `type State int` with `StateIdle, StateChecking, StateLinkOnly, StateDownloading, StateVerifying, StateReady, StateApplying, StateFailed`; `type Event struct { State State; Latest string; Info *UpdateInfo; Ready *ReadyUpdate; Reason string; Err error }`; `type ReadyUpdate struct { Version, Message string; ForceUpdate bool; Staged Staged; Target InstallTarget; Info *UpdateInfo }`; `type Options struct {...}`; `func NewManager(o Options) (*Manager, error)`; `func (m *Manager) Check(ctx context.Context)`; `func (m *Manager) Ready() *ReadyUpdate`; `func (m *Manager) Apply(ctx context.Context) error`

- [ ] **Step 1: Write the failing tests**

`pkg/updater/manager_test.go`:

```go
package updater_test

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"time"

	"aead.dev/minisign"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kpauljoseph/notesankify/pkg/updater"
)

// updateServer serves a manifest, a payload and its signature from memory.
type updateServer struct {
	srv      *httptest.Server
	manifest string
	payload  []byte
	sig      []byte
}

func newUpdateServer(pub minisign.PublicKey, priv minisign.PrivateKey, latest string, payloadPath string) *updateServer {
	us := &updateServer{}
	var err error
	us.payload, err = os.ReadFile(payloadPath)
	Expect(err).NotTo(HaveOccurred())
	us.sig = hashedSignature(priv, us.payload)
	us.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/version.json":
			_, _ = w.Write([]byte(us.manifest))
		case "/NotesAnkify-macos-universal-" + latest + ".zip":
			w.Header().Set("Content-Length", fmt.Sprint(len(us.payload)))
			if r.Method != http.MethodHead {
				_, _ = w.Write(us.payload)
			}
		case "/NotesAnkify-macos-universal-" + latest + ".zip.minisig":
			_, _ = w.Write(us.sig)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	us.manifest = fmt.Sprintf(`{
	  "latest_version": %q,
	  "update_message": "Bug fixes",
	  "force_update": false,
	  "platform_downloads": {"darwin/all": "%s/dmg", "linux/amd64": "%s/tar"},
	  "update_packages": {"darwin/all": {"url": "%s/NotesAnkify-macos-universal-%s.zip", "signature_url": "%s/NotesAnkify-macos-universal-%s.zip.minisig"}}
	}`, latest, us.srv.URL, us.srv.URL, us.srv.URL, latest, us.srv.URL, latest)
	return us
}

// eventLog collects manager events safely across goroutines.
type eventLog struct {
	mu     sync.Mutex
	events []updater.Event
}

func (l *eventLog) add(e updater.Event) { l.mu.Lock(); defer l.mu.Unlock(); l.events = append(l.events, e) }
func (l *eventLog) states() []updater.State {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]updater.State, len(l.events))
	for i, e := range l.events {
		out[i] = e.State
	}
	return out
}
func (l *eventLog) last() updater.Event { l.mu.Lock(); defer l.mu.Unlock(); return l.events[len(l.events)-1] }

var _ = Describe("Manager", func() {
	var (
		dir, installed, payloadPath string
		pub                         minisign.PublicKey
		priv                        minisign.PrivateKey
		us                          *updateServer
		events                      *eventLog
		opts                        updater.Options
		ctx                         = context.Background()
	)

	BeforeEach(func() {
		var err error
		pub, priv, err = minisign.GenerateKey(rand.Reader)
		Expect(err).NotTo(HaveOccurred())
		dir = GinkgoT().TempDir()
		installed = filepath.Join(dir, "Applications", "NotesAnkify.app")
		exe := filepath.Join(installed, "Contents", "MacOS", "notesankify-gui")
		Expect(os.MkdirAll(filepath.Dir(exe), 0o755)).To(Succeed())
		Expect(os.WriteFile(exe, []byte("old"), 0o755)).To(Succeed())
		payloadPath = filepath.Join(dir, "payload.zip")
		writeZip(payloadPath, macAppEntries("new"))
		us = newUpdateServer(pub, priv, "v1.1.0", payloadPath)
		DeferCleanup(us.srv.Close)
		events = &eventLog{}
		pubText, err := pub.MarshalText()
		Expect(err).NotTo(HaveOccurred())
		opts = updater.Options{
			ManifestURL:    us.srv.URL + "/version.json",
			GitHubURL:      us.srv.URL + "/nope",
			PublicKey:      string(pubText),
			StageDir:       filepath.Join(dir, "stage"),
			Client:         us.srv.Client(),
			DownloadClient: us.srv.Client(),
			Probe:          fakeProbe{exe: exe, writable: true, free: 1 << 40},
			Extractor:      updater.ZipExtractor{},
			GOOS:           "darwin",
			GOARCH:         "arm64",
			CurrentVersion: "v1.0.0",
			AutoDownload:   func() bool { return true },
			Log:            updater.NopLogger{},
			OnEvent:        func(e updater.Event) { events.add(e) },
		}
	})

	It("rejects an unparseable public key", func() {
		opts.PublicKey = "nonsense"
		_, err := updater.NewManager(opts)
		Expect(err).To(HaveOccurred())
	})

	It("stays idle when already up to date", func() {
		opts.CurrentVersion = "v1.1.0"
		m, err := updater.NewManager(opts)
		Expect(err).NotTo(HaveOccurred())

		m.Check(ctx)

		Expect(events.states()).To(Equal([]updater.State{updater.StateChecking, updater.StateIdle}))
		Expect(m.Ready()).To(BeNil())
	})

	It("downloads, verifies and reports a ready update", func() {
		m, err := updater.NewManager(opts)
		Expect(err).NotTo(HaveOccurred())

		m.Check(ctx)

		Expect(events.states()).To(Equal([]updater.State{
			updater.StateChecking, updater.StateDownloading, updater.StateVerifying, updater.StateReady,
		}))
		ready := m.Ready()
		Expect(ready).NotTo(BeNil())
		Expect(ready.Version).To(Equal("v1.1.0"))
		Expect(ready.Message).To(Equal("Bug fixes"))
		Expect(ready.Target.Path).To(Equal(installed))
		Expect(ready.Staged.PayloadPath).To(BeAnExistingFile())
		Expect(ready.Info.DownloadURL).To(Equal(us.srv.URL + "/dmg"))
	})

	It("offers only the link when the platform has no package", func() {
		opts.GOOS, opts.GOARCH = "linux", "amd64"
		m, err := updater.NewManager(opts)
		Expect(err).NotTo(HaveOccurred())

		m.Check(ctx)

		last := events.last()
		Expect(last.State).To(Equal(updater.StateLinkOnly))
		Expect(last.Info.DownloadURL).To(Equal(us.srv.URL + "/tar"))
		Expect(last.Info.LatestVersion).To(Equal("1.1.0"))
	})

	It("offers only the link when automatic downloads are off", func() {
		opts.AutoDownload = func() bool { return false }
		m, err := updater.NewManager(opts)
		Expect(err).NotTo(HaveOccurred())

		m.Check(ctx)

		Expect(events.last().State).To(Equal(updater.StateLinkOnly))
		Expect(filepath.Join(opts.StageDir, "v1.1.0")).NotTo(BeADirectory())
	})

	It("offers only the link for developer builds", func() {
		opts.CurrentVersion = "v1.0.0-2-gc638a67"
		m, err := updater.NewManager(opts)
		Expect(err).NotTo(HaveOccurred())

		m.Check(ctx)

		Expect(events.last().State).To(Equal(updater.StateLinkOnly))
		Expect(events.last().Reason).To(ContainSubstring("Developer builds"))
	})

	It("offers only the link with the hint when the app runs from a DMG", func() {
		opts.Probe = fakeProbe{exe: "/Volumes/NotesAnkify/NotesAnkify.app/Contents/MacOS/notesankify-gui", writable: true, free: 1 << 40}
		m, err := updater.NewManager(opts)
		Expect(err).NotTo(HaveOccurred())

		m.Check(ctx)

		Expect(events.last().State).To(Equal(updater.StateLinkOnly))
		Expect(events.last().Reason).To(Equal(updater.MoveToApplicationsHint))
	})

	It("offers only the link when no public key is compiled in", func() {
		opts.PublicKey = ""
		m, err := updater.NewManager(opts)
		Expect(err).NotTo(HaveOccurred())

		m.Check(ctx)

		Expect(events.last().State).To(Equal(updater.StateLinkOnly))
	})

	It("discards a payload with a bad signature and falls back to the link", func() {
		_, other, err := minisign.GenerateKey(rand.Reader)
		Expect(err).NotTo(HaveOccurred())
		us.sig = hashedSignature(other, us.payload)
		m, err := updater.NewManager(opts)
		Expect(err).NotTo(HaveOccurred())

		m.Check(ctx)

		last := events.last()
		Expect(last.State).To(Equal(updater.StateFailed))
		Expect(last.Err).To(MatchError(updater.ErrBadSignature))
		Expect(last.Info.DownloadURL).To(Equal(us.srv.URL + "/dmg"))
		Expect(filepath.Join(opts.StageDir, "v1.1.0")).NotTo(BeADirectory())
		Expect(m.Ready()).To(BeNil())
	})

	It("throttles repeated checks", func() {
		opts.CheckInterval = time.Hour
		m, err := updater.NewManager(opts)
		Expect(err).NotTo(HaveOccurred())

		m.Check(ctx)
		n := len(events.states())
		m.Check(ctx)

		Expect(events.states()).To(HaveLen(n))
	})

	It("falls back to GitHub when the manifest is unreachable", func() {
		gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"tag_name":"v1.1.0","body":"gh","html_url":"https://example.test/rel"}`))
		}))
		DeferCleanup(gh.Close)
		opts.ManifestURL = us.srv.URL + "/missing.json"
		opts.GitHubURL = gh.URL
		m, err := updater.NewManager(opts)
		Expect(err).NotTo(HaveOccurred())

		m.Check(ctx)

		last := events.last()
		Expect(last.State).To(Equal(updater.StateLinkOnly))
		Expect(last.Info.DownloadURL).To(Equal("https://example.test/rel"))
	})

	It("applies a ready update, discards the staged files and reports relaunch trouble", func() {
		m, err := updater.NewManager(opts)
		Expect(err).NotTo(HaveOccurred())
		m.Check(ctx)
		Expect(m.Ready()).NotTo(BeNil())

		err = m.Apply(ctx)

		// Relaunch is unsupported on the test platform, so the install
		// succeeds and the relaunch is reported.
		Expect(errors.Is(err, updater.ErrRelaunch)).To(BeTrue())
		Expect(os.ReadFile(filepath.Join(installed, "Contents", "MacOS", "notesankify-gui"))).To(Equal([]byte("new")))
		Expect(filepath.Join(opts.StageDir, "v1.1.0")).NotTo(BeADirectory())
		Expect(events.last().State).To(Equal(updater.StateApplying))
	})

	It("refuses to apply when nothing is ready", func() {
		m, err := updater.NewManager(opts)
		Expect(err).NotTo(HaveOccurred())
		Expect(m.Apply(ctx)).To(MatchError(ContainSubstring("no update is ready")))
	})
})
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./pkg/updater/`
Expected: FAIL with `undefined: updater.Options`.

- [ ] **Step 3: Implement manager.go**

```go
package updater

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"aead.dev/minisign"
)

// State is where the manager is in the update pipeline.
type State int

const (
	StateIdle State = iota
	StateChecking
	StateLinkOnly    // a newer version exists but can only be offered as a link
	StateDownloading
	StateVerifying
	StateReady       // a verified payload is staged; waiting for the user
	StateApplying
	StateFailed      // download, verification or apply failed; link offered
)

func (s State) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateChecking:
		return "checking"
	case StateLinkOnly:
		return "link-only"
	case StateDownloading:
		return "downloading"
	case StateVerifying:
		return "verifying"
	case StateReady:
		return "ready"
	case StateApplying:
		return "applying"
	case StateFailed:
		return "failed"
	}
	return fmt.Sprintf("state(%d)", int(s))
}

// Event is what the GUI receives on every transition.
type Event struct {
	State  State
	Latest string       // latest version from the manifest, when known
	Info   *UpdateInfo  // what to show and where to send the user (LinkOnly, Failed, Ready)
	Ready  *ReadyUpdate // the staged update (Ready, Applying)
	Reason string       // one line for the user (LinkOnly, Failed)
	Err    error
}

// ReadyUpdate is a verified, staged update waiting for a restart.
type ReadyUpdate struct {
	Version     string
	Message     string
	ForceUpdate bool
	Staged      Staged
	Target      InstallTarget
	Info        *UpdateInfo
}

// Options configures a Manager. Zero values get sensible defaults for the
// running program; tests set everything explicitly.
type Options struct {
	ManifestURL    string
	GitHubURL      string
	PublicKey      string // minisign public key; empty disables in-place updates
	StageDir       string
	Client         *http.Client // manifest and HEAD requests
	DownloadClient *http.Client // payload downloads
	Probe          Probe
	Extractor      Extractor
	GOOS, GOARCH   string
	CurrentVersion string
	AutoDownload   func() bool
	Log            Logger
	OnEvent        func(Event)
	CheckInterval  time.Duration // minimum time between checks; default one hour
}

// Manager runs the update pipeline. One check or apply runs at a time.
type Manager struct {
	o      Options
	pub    minisign.PublicKey
	keyOK  bool
	stager *Stager

	mu    sync.Mutex
	busy  bool
	last  time.Time
	ready *ReadyUpdate
}

// NewManager applies defaults and parses the public key.
func NewManager(o Options) (*Manager, error) {
	if o.ManifestURL == "" {
		o.ManifestURL = ManifestURL()
	}
	if o.GitHubURL == "" {
		o.GitHubURL = DefaultGitHubLatestURL
	}
	if o.Client == nil {
		o.Client = &http.Client{Timeout: 10 * time.Second}
	}
	if o.DownloadClient == nil {
		o.DownloadClient = NewDownloadClient()
	}
	if o.Probe == nil {
		o.Probe = OSProbe{}
	}
	if o.Extractor == nil {
		o.Extractor = DefaultExtractor()
	}
	if o.GOOS == "" {
		o.GOOS = runtime.GOOS
	}
	if o.GOARCH == "" {
		o.GOARCH = runtime.GOARCH
	}
	if o.AutoDownload == nil {
		o.AutoDownload = func() bool { return true }
	}
	if o.Log == nil {
		o.Log = NopLogger{}
	}
	if o.CheckInterval == 0 {
		o.CheckInterval = time.Hour
	}
	m := &Manager{o: o}
	if strings.TrimSpace(o.PublicKey) != "" {
		pub, err := ParsePublicKey(o.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("release public key: %w", err)
		}
		m.pub = pub
		m.keyOK = true
	}
	m.stager = &Stager{Dir: o.StageDir, Client: o.DownloadClient, MaxBytes: DefaultMaxPayloadBytes, Log: o.Log}
	return m, nil
}

// Ready returns the staged update awaiting a restart, if any.
func (m *Manager) Ready() *ReadyUpdate {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ready
}

// Check runs one pass: fetch the manifest, and if a newer release exists,
// stage and verify its payload. It returns at once when a pass is already
// running or the previous one started less than CheckInterval ago.
func (m *Manager) Check(ctx context.Context) {
	if !m.begin(true) {
		return
	}
	defer m.end()

	m.emit(Event{State: StateChecking})
	manifest, err := FetchManifest(ctx, m.o.Client, m.o.ManifestURL)
	if err != nil {
		m.o.Log.Debug("Manifest unavailable (%v); trying GitHub", err)
		manifest, err = FetchGitHubLatest(ctx, m.o.Client, m.o.GitHubURL)
		if err != nil {
			m.o.Log.Debug("Update check failed: %v", err)
			m.emit(Event{State: StateIdle, Err: err})
			return
		}
	}

	latest := Normalize(manifest.LatestVersion)
	if !IsNewer(m.o.CurrentVersion, latest) {
		m.emit(Event{State: StateIdle, Latest: latest})
		return
	}
	info := &UpdateInfo{
		CurrentVersion: strings.TrimPrefix(Normalize(m.o.CurrentVersion), "v"),
		LatestVersion:  strings.TrimPrefix(latest, "v"),
		UpdateMessage:  manifest.UpdateMessage,
		DownloadURL:    manifest.DownloadFor(m.o.GOOS, m.o.GOARCH),
		IsAvailable:    true,
		ForceUpdate:    manifest.ForceUpdate,
	}
	linkOnly := func(reason string) {
		m.emit(Event{State: StateLinkOnly, Latest: latest, Info: info, Reason: reason})
	}
	fail := func(reason string, err error) {
		m.o.Log.Info("Update %s: %s (%v)", latest, reason, err)
		m.emit(Event{State: StateFailed, Latest: latest, Info: info, Reason: reason, Err: err})
	}

	pkg, ok := manifest.PackageFor(m.o.GOOS, m.o.GOARCH)
	switch {
	case !ok, !m.keyOK, m.o.StageDir == "":
		linkOnly("")
		return
	case !IsRelease(m.o.CurrentVersion):
		linkOnly("Developer builds do not update automatically.")
		return
	case !m.o.AutoDownload():
		linkOnly("")
		return
	}

	target, err := FindTarget(m.o.GOOS, m.o.Probe)
	if err != nil {
		m.o.Log.Debug("No in-place target: %v", err)
		linkOnly("")
		return
	}
	if err := os.MkdirAll(m.o.StageDir, 0o755); err != nil {
		m.o.Log.Debug("Cannot create staging folder: %v", err)
		linkOnly("")
		return
	}
	size, err := m.stager.Size(ctx, pkg.URL)
	if err != nil {
		size = DefaultMaxPayloadBytes
	}
	if reason, ok := CheckInPlace(m.o.GOOS, target, m.o.Probe, m.o.StageDir, size); !ok {
		linkOnly(reason)
		return
	}

	m.emit(Event{State: StateDownloading, Latest: latest})
	staged, err := m.stager.Stage(ctx, latest, pkg)
	if err != nil {
		fail("The update could not be downloaded. It will be retried later.", err)
		return
	}

	m.emit(Event{State: StateVerifying, Latest: latest})
	if err := VerifyFile(m.pub, staged.PayloadPath, staged.SigPath); err != nil {
		_ = m.stager.Discard(latest)
		fail("The downloaded update could not be verified.", err)
		return
	}
	if err := CheckPayload(m.o.GOOS, staged.PayloadPath); err != nil {
		_ = m.stager.Discard(latest)
		fail("The downloaded update is not a NotesAnkify package.", err)
		return
	}

	ready := &ReadyUpdate{
		Version:     latest,
		Message:     manifest.UpdateMessage,
		ForceUpdate: manifest.ForceUpdate,
		Staged:      staged,
		Target:      target,
		Info:        info,
	}
	m.mu.Lock()
	m.ready = ready
	m.mu.Unlock()
	m.emit(Event{State: StateReady, Latest: latest, Info: info, Ready: ready})
}

// Apply installs the ready update and starts the new build. When it
// returns nil, or an error wrapping ErrRelaunch, the caller must exit.
func (m *Manager) Apply(ctx context.Context) error {
	m.mu.Lock()
	ready := m.ready
	m.mu.Unlock()
	if ready == nil {
		return errors.New("no update is ready")
	}
	if !m.begin(false) {
		return errors.New("an update check is still running; try again in a moment")
	}
	defer m.end()

	m.emit(Event{State: StateApplying, Latest: ready.Version, Ready: ready, Info: ready.Info})

	// The staged file has sat on disk since it was verified; check it again.
	if err := VerifyFile(m.pub, ready.Staged.PayloadPath, ready.Staged.SigPath); err != nil {
		_ = m.stager.Discard(ready.Version)
		m.clearReady()
		m.emit(Event{State: StateFailed, Latest: ready.Version, Info: ready.Info, Reason: "The downloaded update could not be verified.", Err: err})
		return err
	}
	applier := &Applier{GOOS: m.o.GOOS, Extractor: m.o.Extractor, Log: m.o.Log}
	if err := applier.Apply(ready.Target, ready.Staged.PayloadPath); err != nil {
		m.emit(Event{State: StateFailed, Latest: ready.Version, Info: ready.Info, Reason: "The update could not be installed.", Err: err})
		return err
	}
	_ = m.stager.Discard(ready.Version)
	m.clearReady()
	if err := Relaunch(ready.Target); err != nil {
		m.o.Log.Info("Update installed but relaunch failed: %v", err)
		return fmt.Errorf("%w: %v", ErrRelaunch, err)
	}
	return nil
}

// begin claims the manager. With throttle set, it also refuses to start
// a check within CheckInterval of the previous one.
func (m *Manager) begin(throttle bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.busy {
		return false
	}
	now := time.Now()
	if throttle && !m.last.IsZero() && now.Sub(m.last) < m.o.CheckInterval {
		return false
	}
	if throttle {
		m.last = now
	}
	m.busy = true
	return true
}

func (m *Manager) end() {
	m.mu.Lock()
	m.busy = false
	m.mu.Unlock()
}

func (m *Manager) clearReady() {
	m.mu.Lock()
	m.ready = nil
	m.mu.Unlock()
}

func (m *Manager) emit(e Event) {
	m.o.Log.Debug("Updater: %s", e.State)
	if m.o.OnEvent != nil {
		m.o.OnEvent(e)
	}
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./pkg/updater/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/updater/manager.go pkg/updater/manager_test.go
git commit -s -m "feat(updater): add the check, stage, verify, ready, apply state machine"
```

### Task 11: GUI wiring

**Files:**
- Create: `pkg/updater/pubkey.go`
- Delete: `pkg/updater/checker.go`
- Modify: `cmd/notesankify-gui/main.go`
- Modify: `pkg/utils/browse_test.go` (Windows-safe HOME override for CI)

**Interfaces:**
- Consumes: `Manager`, `Options`, `Event`, `ReadyUpdate`, states, `ErrRelaunch`, `RemoveLeftovers`, `OSProbe`, `DefaultStageDir`, `PublicKey` (this task)

The GUI has no automated tests; verification is `go vet` with the x11 tag and a manual run on the Mac.

- [ ] **Step 1: Create pubkey.go**

```go
package updater

// PublicKey is the minisign public key that release payloads are signed
// with: the second line of ~/.minisign/minisign.pub on the release machine,
// the one starting with "RW". An empty value disables in-place updates;
// the app then only offers the download link.
const PublicKey = ""
```

When the maintainer has generated the key, replace `""` with the key line in quotes, for example `"RWQf6LRCGA9i53mlYecO4IzT51TGPpvWucNSCh1CBM0QTaLn73Y7GFd3"`, and commit that change separately with the message `feat(updater): compile in the release signing public key`.

- [ ] **Step 2: Delete checker.go**

```bash
git rm pkg/updater/checker.go
```

- [ ] **Step 3: Edit the imports and constants in main.go**

In the import block add these three (the file already imports `context` and `runtime`; do not duplicate those):

```go
	"errors"
	"strings"
	"sync/atomic"
```

Add these constants right after the `ProcessingMode` constants block near the top of the file (search for `ModeBoth`):

```go
const (
	appID           = "com.notesankify.app"
	prefAutoDownload = "updates.auto_download"
)
```

- [ ] **Step 4: Change the struct**

In `type NotesAnkifyGUI struct`, replace the line `updateChecker *updater.Checker` with:

```go
	app           fyne.App
	updates       *updater.Manager
	processing    atomic.Bool
	pendingReady  *updater.ReadyUpdate
```

and in the `// UI components` group add:

```go
	autoUpdateCheck *widget.Check
```

- [ ] **Step 5: Change app creation and cleanup in NewNotesAnkifyGUI**

Replace `notesankifyApp := app.New()` with:

```go
	notesankifyApp := app.NewWithID(appID)
```

After the `filerepo.Register()` block add:

```go
	if err := updater.RemoveLeftovers(runtime.GOOS, updater.OSProbe{}); err != nil {
		log.Info("Could not remove the previous version's leftovers: %v", err)
	}
```

In the returned struct literal replace `updateChecker:  updater.NewChecker(log),` with `app:            notesankifyApp,` and, after the `return &NotesAnkifyGUI{...}` is built, wire the manager. The function currently ends with `return &NotesAnkifyGUI{ ... }`; change it to:

```go
	gui := &NotesAnkifyGUI{
		window:         window,
		app:            notesankifyApp,
		log:            log,
		scanner:        scanner.New(log),
		ankiService:    anki.NewService(log),
		logFileName:    logFileName,
		dimensions:     dimensions,
		processingMode: ModeBoth, // Start with most strict mode
	}
	gui.setupUpdater()
	return gui
}

// setupUpdater builds the update manager. Every event it emits is handed
// to the UI goroutine.
func (gui *NotesAnkifyGUI) setupUpdater() {
	stageDir, err := updater.DefaultStageDir()
	if err != nil {
		gui.log.Info("No cache folder for updates, in-place updates disabled: %v", err)
	}
	m, err := updater.NewManager(updater.Options{
		PublicKey:      updater.PublicKey,
		StageDir:       stageDir,
		CurrentVersion: version.Version,
		AutoDownload: func() bool {
			return gui.app.Preferences().BoolWithFallback(prefAutoDownload, true)
		},
		Log: gui.log,
		OnEvent: func(e updater.Event) {
			fyne.Do(func() { gui.handleUpdateEvent(e) })
		},
	})
	if err != nil {
		gui.log.Info("Automatic updates disabled: %v", err)
		return
	}
	gui.updates = m
}
```

- [ ] **Step 6: Add the preference checkbox in setupUI**

Directly after the `gui.verboseCheck = widget.NewCheck(...)` statement add:

```go
	gui.autoUpdateCheck = widget.NewCheck("Download updates automatically", func(checked bool) {
		gui.app.Preferences().SetBool(prefAutoDownload, checked)
	})
	gui.autoUpdateCheck.SetChecked(gui.app.Preferences().BoolWithFallback(prefAutoDownload, true))
```

and change the container that holds the verbose checkbox, `container.NewVBox(gui.verboseCheck))`, to:

```go
		container.NewVBox(gui.verboseCheck, gui.autoUpdateCheck))
```

- [ ] **Step 7: Track processing so the ready dialog waits**

In `handleProcess`, immediately before `go gui.processFiles(gui.dirEntry.Text, gui.rootDeckEntry.Text)` add:

```go
	gui.processing.Store(true)
```

In `processFiles`, replace the deferred block

```go
	defer fyne.Do(func() {
		gui.progress.Hide()
	})
```

with

```go
	defer fyne.Do(func() {
		gui.progress.Hide()
		gui.processing.Store(false)
		if gui.pendingReady != nil {
			gui.showReadyDialog(gui.pendingReady)
		}
	})
```

- [ ] **Step 8: Replace the update checker functions**

Replace the whole of `checkForUpdates` and `showUpdateDialog` (keep `startUpdateChecker` and `openBrowser`) with:

```go
func (gui *NotesAnkifyGUI) checkForUpdates() {
	if gui.updates == nil {
		return
	}
	gui.updates.Check(context.Background())
}

// handleUpdateEvent runs on the UI goroutine.
func (gui *NotesAnkifyGUI) handleUpdateEvent(e updater.Event) {
	switch e.State {
	case updater.StateLinkOnly, updater.StateFailed:
		if e.Info != nil {
			gui.showUpdateDialog(e.Info, e.Reason)
		}
	case updater.StateReady:
		if gui.processing.Load() {
			gui.pendingReady = e.Ready
			return
		}
		gui.showReadyDialog(e.Ready)
	}
}

// showUpdateDialog is the link-only dialog: the user downloads by hand.
func (gui *NotesAnkifyGUI) showUpdateDialog(info *updater.UpdateInfo, reason string) {
	message := fmt.Sprintf(
		"A new version of NotesAnkify is available!\n\n"+
			"Current version: %s\n"+
			"Latest version: %s\n\n"+
			"%s",
		info.CurrentVersion,
		info.LatestVersion,
		info.UpdateMessage,
	)
	if reason != "" {
		message += "\n\n" + reason
	}

	content := container.NewVBox(
		widget.NewRichTextFromMarkdown(message),
		container.NewHBox(
			widget.NewButton("Download Update", func() {
				gui.openBrowser(info.DownloadURL)
			}),
		),
	)

	var d dialog.Dialog
	if info.ForceUpdate {
		message = "This update is required. Please update to continue using NotesAnkify.\n\n" + message
		d = dialog.NewCustom("Required Update Available", "", content, gui.window)
	} else {
		d = dialog.NewCustom("Update Available", "Later", content, gui.window)
	}
	d.Resize(fyne.NewSize(500, 300))
	d.Show()
}

// showReadyDialog offers a staged, verified update.
func (gui *NotesAnkifyGUI) showReadyDialog(ready *updater.ReadyUpdate) {
	gui.pendingReady = nil
	message := fmt.Sprintf("NotesAnkify %s is ready to install.\n\n%s",
		strings.TrimPrefix(ready.Version, "v"), ready.Message)
	content := widget.NewRichTextFromMarkdown(message)

	if ready.ForceUpdate {
		d := dialog.NewCustom("Update required", "Restart now", content, gui.window)
		d.SetOnClosed(gui.applyUpdate)
		d.Resize(fyne.NewSize(500, 240))
		d.Show()
		return
	}
	d := dialog.NewCustomConfirm("Update ready", "Restart now", "Later", content, func(install bool) {
		if install {
			gui.applyUpdate()
		}
	}, gui.window)
	d.Resize(fyne.NewSize(500, 240))
	d.Show()
}

// applyUpdate installs the staged update off the UI goroutine and quits
// once the new build has been started.
func (gui *NotesAnkifyGUI) applyUpdate() {
	gui.updateStatus("Installing update...")
	go func() {
		err := gui.updates.Apply(context.Background())
		fyne.Do(func() {
			if err != nil && !errors.Is(err, updater.ErrRelaunch) {
				gui.updateStatus("Ready to process files...")
				dialog.ShowError(fmt.Errorf("the update could not be installed: %v", err), gui.window)
				return
			}
			if err != nil {
				gui.log.Info("%v", err)
			}
			gui.app.Quit()
		})
	}()
}
```

- [ ] **Step 9: Make the utils test Windows-safe for CI**

In `pkg/utils/browse_test.go`, in the `BeforeEach`, after `GinkgoT().Setenv("HOME", home)` add:

```go
		GinkgoT().Setenv("USERPROFILE", home)
```

- [ ] **Step 10: Vet and test**

Run:
```bash
gofmt -l cmd pkg internal
go vet -tags x11 ./...
go test ./pkg/... ./internal/filerepo/... ./internal/scanner/...
```
Expected: gofmt prints nothing, vet is clean (on Linux this needs the X11/GL headers as documented in the repo's CI notes; on the Mac plain `go vet ./...` works), tests PASS.

- [ ] **Step 11: Manual check on the Mac**

Build with `make generate-version bundle-assets` then the `fyne package` command from RELEASING.md, launch with `open ./NotesAnkify.app`, and confirm: the new checkbox appears and its state survives a relaunch; with `NOTESANKIFY_UPDATE_URL` pointed at a local manifest whose `latest_version` is higher, the "Update ready" dialog appears after the download; Later closes it; Restart now swaps the bundle and relaunches.

- [ ] **Step 12: Commit**

```bash
git add pkg/updater/pubkey.go cmd/notesankify-gui/main.go pkg/utils/browse_test.go
git commit -s -m "feat(gui): download updates in the background and offer a one-click restart"
```

### Task 12: CI for the pure-Go packages on three OSes

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Write the workflow**

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

permissions:
  contents: read

jobs:
  test:
    name: test (${{ matrix.os }})
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      # Only packages without cgo or a GUI dependency: they run on plain
      # runners and give the Windows executable swap a real Windows run.
      - run: go test -count=1 ./pkg/updater/... ./pkg/utils/... ./pkg/models/... ./internal/filerepo/... ./internal/scanner/...
      - run: go vet ./pkg/updater/... ./pkg/utils/... ./internal/filerepo/...
```

- [ ] **Step 2: Validate the YAML locally**

Run: `ruby -ryaml -e 'puts YAML.load_file(".github/workflows/ci.yml")["jobs"].keys'`
Expected: `test`. (Ruby ships with macOS; on Linux `python3 -c "import yaml"` works when PyYAML is installed.)

- [ ] **Step 3: Commit and push, then watch the three runners go green**

```bash
git add .github/workflows/ci.yml
git commit -s -m "ci: test the updater and other pure-Go packages on Linux, macOS and Windows"
git push -u origin feat/seamless-updates
gh run watch
```

Expected: three green jobs. If the Windows job fails in `internal/filerepo`, the failing spec will be the execute-only directory one (Windows has no execute-only mode); wrap that spec's chmod and assertion in `if runtime.GOOS != "windows"` and commit `fix(filerepo): skip the execute-only directory spec on Windows`.

### Task 13: Changelog for Part A

**Files:**
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Add entries under `## [Unreleased]`**

Under `### Added`, create the heading if missing and add:

```markdown
- One-click updates on macOS and Windows. The app downloads the next
  release in the background, verifies its signature, and offers
  "Restart now". Linux keeps the download link. Turn background downloads
  off with the "Download updates automatically" checkbox.
- The app now identifies itself with its version and platform when it
  checks notesankify.com for updates.
```

Under `### Changed` add:

```markdown
- Version comparison is now numeric, so 1.10 sorts after 1.9.
- The app registers with the desktop under its bundle identifier, so
  preferences persist and the folder dialog remembers the last folder.
```

- [ ] **Step 2: Commit**

```bash
git add CHANGELOG.md
git commit -s -m "docs: changelog entries for one-click updates"
```

---

# Part B: Release tooling and documents

Branch: `build/release-signing`, from main after Part A is merged (it edits `publish-version.yml`, which must already exist on main).

### Task 14: Fix bundle-assets and ignore generated files

**Files:**
- Modify: `Makefile` (target `bundle-assets`)
- Modify: `.gitignore`

- [ ] **Step 1: Replace the bundle-assets target**

Current:

```makefile
bundle-assets:
	mkdir -p $(ASSETS_BUNDLE_DIR)
	fyne bundle -o $(ASSETS_BUNDLE_DIR)/bundled.go --package bundle --prefix Resource assets/icons/png/icon-256.png
```

New (the current fyne CLI emits a go:embed path relative to the package directory, so the icon is copied next to the generated file and the command runs from there):

```makefile
bundle-assets:
	mkdir -p $(ASSETS_BUNDLE_DIR)
	cp $(ASSETS_ICONS)/png/icon-256.png $(ASSETS_BUNDLE_DIR)/icon-256.png
	cd $(ASSETS_BUNDLE_DIR) && fyne bundle -o bundled.go --package bundle --prefix Resource icon-256.png
```

- [ ] **Step 2: Ignore the generated folder**

Append to `.gitignore`:

```
# Generated by make bundle-assets
assets/bundle/
```

- [ ] **Step 3: Verify**

Run: `make bundle-assets && grep -n 'go:embed\|^var Resource' assets/bundle/bundled.go && go build ./cmd/notesankify-gui/ && git status --short assets`
Expected: the embed line reads `//go:embed icon-256.png`, the variable is `ResourceIcon256Png`, the build succeeds (on the Mac; on Linux use the vet command from Task 11 instead), and `git status` shows nothing under `assets/`.

- [ ] **Step 4: Commit**

```bash
git add Makefile .gitignore
git commit -s -m "build: generate the asset bundle from its own folder and ignore it"
```

### Task 15: Sign the macOS bundle, zip it, and sign the update payloads

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add variables**

After the line `DMG_DIR := $(DIST_DIR)/darwin-dmg` add:

```makefile
DARWIN_ZIP_DIR := $(DIST_DIR)/darwin-zip
MINISIGN ?= minisign
UPDATE_PAYLOADS := \
	$(DARWIN_ZIP_DIR)/$(APP_NAME)-macos-universal-$(VERSION).zip \
	$(DIST_DIR)/windows-amd64/$(APP_NAME)-windows-amd64-$(VERSION).zip \
	$(DIST_DIR)/windows-arm64/$(APP_NAME)-windows-arm64-$(VERSION).zip
```

- [ ] **Step 2: Add the signing and zip targets, and re-point darwin-dmg**

Insert after the `darwin-universal` target:

```makefile
darwin-sign: darwin-universal
	@echo "Signing macOS bundle with a stable identifier requirement..."
	codesign --force --sign - --identifier $(BUNDLE_ID) \
		-r '=designated => identifier "$(BUNDLE_ID)"' \
		$(DARWIN_UNIVERSAL_DIR)/$(APP_NAME).app
	codesign --verify --verbose $(DARWIN_UNIVERSAL_DIR)/$(APP_NAME).app

darwin-zip: darwin-sign
	@echo "Zipping macOS bundle for in-app updates..."
	mkdir -p $(DARWIN_ZIP_DIR)
	ditto -c -k --keepParent $(DARWIN_UNIVERSAL_DIR)/$(APP_NAME).app \
		$(DARWIN_ZIP_DIR)/$(APP_NAME)-macos-universal-$(VERSION).zip
```

Change the `darwin-dmg` dependency line from `darwin-dmg: darwin-universal` to `darwin-dmg: darwin-sign`.

Insert after the `linux-app` target:

```makefile
sign-payloads: darwin-zip windows-app
	@echo "Signing update payloads with minisign (the key passphrase is asked once)..."
	$(MINISIGN) -Sm $(UPDATE_PAYLOADS)
```

Change `package-all` to:

```makefile
package-all: clean bundle-assets generate-version darwin-dmg darwin-zip windows-app linux-app sign-payloads
```

Add `darwin-sign darwin-zip sign-payloads` to the `.PHONY` line.

- [ ] **Step 3: Verify on the Mac**

Run: `make darwin-zip && codesign -d -r- fyne-cross/dist/darwin-universal/NotesAnkify.app && unzip -l fyne-cross/dist/darwin-zip/*.zip | head`
Expected: the requirement prints `designated => identifier "com.notesankify.app"`, and the zip listing starts with `NotesAnkify.app/Contents/`.

Then: `make sign-payloads` (after `make windows-app`) and `ls fyne-cross/dist/*/*.minisig`
Expected: three `.minisig` files.

- [ ] **Step 4: Commit**

```bash
git add Makefile
git commit -s -m "build: sign the macOS bundle, zip it for updates, and minisign the payloads"
```

### Task 16: Upload the new assets

**Files:**
- Modify: `upload-release.sh`

- [ ] **Step 1: Add the uploads**

After the block that uploads the macOS DMG add:

```bash
# Upload macOS update payload and its signature
gh release upload "${VERSION}" \
    "fyne-cross/dist/darwin-zip/NotesAnkify-macos-universal-${VERSION}.zip" \
    "fyne-cross/dist/darwin-zip/NotesAnkify-macos-universal-${VERSION}.zip.minisig" \
    --clobber
```

After the block that uploads the Windows zips add:

```bash
# Upload Windows payload signatures
gh release upload "${VERSION}" \
    "fyne-cross/dist/windows-amd64/NotesAnkify-windows-amd64-${VERSION}.zip.minisig" \
    "fyne-cross/dist/windows-arm64/NotesAnkify-windows-arm64-${VERSION}.zip.minisig" \
    --clobber
```

- [ ] **Step 2: Verify**

Run: `bash -n upload-release.sh && grep -c minisig upload-release.sh`
Expected: no syntax error, count 3.

- [ ] **Step 3: Commit**

```bash
git add upload-release.sh
git commit -s -m "build: upload the macOS zip and minisign signatures with each release"
```

### Task 17: Publish update packages in version.json

**Files:**
- Modify: `.github/workflows/publish-version.yml`

- [ ] **Step 1: Extend the generated JSON**

In the heredoc that writes `temp-version/version.json`, replace the `platform_downloads` block's closing `}` and the object's closing so the file reads:

```yaml
          cat > temp-version/version.json << EOF
          {
              "latest_version": "${VERSION}",
              "min_version": "${VERSION}",
              "update_message": "New version ${VERSION} is available with improvements and bug fixes!",
              "force_update": false,
              "platform_downloads": {
                  "windows/amd64": "https://github.com/kpauljoseph/notesankify/releases/download/${VERSION}/NotesAnkify-windows-amd64-${VERSION}.zip",
                  "windows/arm64": "https://github.com/kpauljoseph/notesankify/releases/download/${VERSION}/NotesAnkify-windows-arm64-${VERSION}.zip",
                  "darwin/all": "https://github.com/kpauljoseph/notesankify/releases/download/${VERSION}/NotesAnkify-macos-universal-${VERSION}.dmg",
                  "linux/amd64": "https://github.com/kpauljoseph/notesankify/releases/download/${VERSION}/NotesAnkify-linux-amd64-${VERSION}.tar.xz"
              },
              "update_packages": {
                  "darwin/all": {
                      "url": "https://github.com/kpauljoseph/notesankify/releases/download/${VERSION}/NotesAnkify-macos-universal-${VERSION}.zip",
                      "signature_url": "https://github.com/kpauljoseph/notesankify/releases/download/${VERSION}/NotesAnkify-macos-universal-${VERSION}.zip.minisig"
                  },
                  "windows/amd64": {
                      "url": "https://github.com/kpauljoseph/notesankify/releases/download/${VERSION}/NotesAnkify-windows-amd64-${VERSION}.zip",
                      "signature_url": "https://github.com/kpauljoseph/notesankify/releases/download/${VERSION}/NotesAnkify-windows-amd64-${VERSION}.zip.minisig"
                  },
                  "windows/arm64": {
                      "url": "https://github.com/kpauljoseph/notesankify/releases/download/${VERSION}/NotesAnkify-windows-arm64-${VERSION}.zip",
                      "signature_url": "https://github.com/kpauljoseph/notesankify/releases/download/${VERSION}/NotesAnkify-windows-arm64-${VERSION}.zip.minisig"
                  }
              }
          }
          EOF
```

- [ ] **Step 2: Verify the JSON the workflow would produce**

Run:
```bash
python3 - <<'PY'
import json, re
text = open(".github/workflows/publish-version.yml").read()
block = re.search(r"<< EOF\n(.*?)\n\s*EOF\n", text, re.S).group(1)
body = "\n".join(line[10:] for line in block.splitlines()).replace("${VERSION}", "v9.9.9")
doc = json.loads(body)
print(sorted(doc["update_packages"]))
PY
```
Expected: `['darwin/all', 'windows/amd64', 'windows/arm64']`. Any JSON syntax slip in the heredoc raises here instead of at release time.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/publish-version.yml
git commit -s -m "ci: publish signed update package URLs in version.json"
```

### Task 18: RELEASING.md, docs and changelog

**Files:**
- Create: `RELEASING.md`
- Modify: `docs/README.md`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Write RELEASING.md**

```markdown
# Releasing NotesAnkify

## One-time setup on the release Mac

- Docker Desktop or Rancher Desktop running (fyne-cross builds in containers).
- `go install github.com/fyne-io/fyne-cross@latest`
- `go install fyne.io/tools/cmd/fyne@latest`
- `brew install minisign`, then once: `minisign -G`. Choose a passphrase.
  Store `~/.minisign/minisign.key` and the passphrase in your password
  manager. Losing the key means no installed copy can update itself again.
- Put the second line of `~/.minisign/minisign.pub` (starts with `RW`) into
  `pkg/updater/pubkey.go` as the `PublicKey` constant, if not already done.
- `gh auth login`
- `wrangler login` (only needed to change the Cloudflare Worker).

## Per release

1. Merge everything to `main`. In `CHANGELOG.md` rename `## [Unreleased]`
   to `## [X.Y.Z] - YYYY-MM-DD`. The release workflow extracts the notes by
   matching that heading exactly.

2. Tag and push:

       git tag vX.Y.Z && git push origin vX.Y.Z

   `release.yml` creates a draft release with those notes.

3. Build at the tag, so the compiled version is exactly the tag:

       git checkout vX.Y.Z && make package-all

   This builds every platform, signs the macOS bundle with the stable
   identifier requirement, zips it, and signs the three update payloads
   with minisign (one passphrase prompt).

4. Upload:

       GITHUB_TOKEN=$(gh auth token) ./upload-release.sh vX.Y.Z

5. Smoke test the DMG from the draft release on this Mac: the version in
   the app header, `codesign -d -r- /Applications/NotesAnkify.app` shows
   `designated => identifier "com.notesankify.app"`, and Browse opens
   without privacy prompts.

6. Rehearse the in-app update from the previous release (see below).

7. Publish the release on GitHub. `publish-version.yml` then updates
   `version.json` on notesankify.com. Confirm after a few minutes (the edge
   cache is five minutes):

       curl -s https://notesankify.com/version.json | python3 -m json.tool

8. Next day, run `cloudflare/stats.sql` (see `cloudflare/README.md`) and
   watch the new version's share climb.

## Rehearsing the update path

Draft-release assets are not publicly downloadable, so the rehearsal
serves the freshly built payloads from this Mac.

1. Create a folder with the payloads, their signatures, and a manifest:

       mkdir -p /tmp/rehearsal && cd /tmp/rehearsal
       cp <repo>/fyne-cross/dist/darwin-zip/NotesAnkify-macos-universal-vX.Y.Z.zip* .
       cp <repo>/fyne-cross/dist/windows-*/NotesAnkify-windows-*-vX.Y.Z.zip* .

   Write `version.json` with `latest_version` set to `vX.Y.Z`, and every
   URL in `platform_downloads` and `update_packages` pointing at
   `http://localhost:8000/<file name>`.

2. Serve it:

       python3 -m http.server 8000

3. Launch the previous release with the override:

       NOTESANKIFY_UPDATE_URL=http://localhost:8000/version.json open -n /Applications/NotesAnkify.app

4. Expect, within about ten seconds: a background download, then the
   "Update ready" dialog. Click Restart now. The app relaunches as the new
   version; `~/notesankify-logs` has the updater's log lines; the
   `NotesAnkify.app.old` folder next to the app is gone after the relaunch.

5. Run the same on a Windows machine when one is available. CI already
   exercises the executable swap on GitHub's Windows runners.

## When something goes wrong

- A user reports "The downloaded update could not be verified": the
  payload on GitHub does not match its `.minisig`. Re-run
  `make sign-payloads` and the upload script; do not change the key.
- Users on a mounted DMG or translocated copy see the hint to move the app
  to Applications. That is expected.
- To take an update back, publish a new release with a higher version. Do
  not delete or edit assets of a published release; installs may be
  mid-download.
```

- [ ] **Step 2: Mention the default folder in the user docs**

Run `grep -n -i 'browse' docs/README.md` to find the step that describes choosing the PDF folder, and add this sentence to that step:

```markdown
The folder picker opens in a `NotesAnkify` folder inside your home folder, which the app creates for you; keeping your PDFs there means macOS never asks for folder permissions.
```

- [ ] **Step 3: Changelog**

Under `## [Unreleased]`, `### Added`, append:

```markdown
- Release payloads are signed with minisign and the macOS bundle carries a
  stable code-signing identity, so folder permissions survive updates.
- `RELEASING.md` documents the release process end to end.
```

- [ ] **Step 4: Commit**

```bash
git add RELEASING.md docs/README.md CHANGELOG.md
git commit -s -m "docs: add the release checklist and document the default folder"
```

---

# Part C: Usage metrics on Cloudflare

Branch: `feat/usage-metrics`. Independent of Parts A and B in code; the numbers become richer once Part A ships the descriptive User-Agent.

### Task 19: The Worker and its User-Agent parser

**Files:**
- Create: `cloudflare/worker.js`
- Create: `cloudflare/worker.test.mjs`

- [ ] **Step 1: Write the failing test**

`cloudflare/worker.test.mjs`:

```js
import test from "node:test";
import assert from "node:assert/strict";
import { parseAgent } from "./worker.js";

test("parses the app's User-Agent", () => {
  assert.deepEqual(parseAgent("NotesAnkify/1.1.0 (darwin; arm64)"), {
    client: "app", version: "1.1.0", os: "darwin", arch: "arm64",
  });
});

test("classifies the 1.0.0 Go client as legacy", () => {
  assert.deepEqual(parseAgent("Go-http-client/1.1"), {
    client: "legacy", version: "legacy", os: "unknown", arch: "unknown",
  });
});

test("ignores browsers and crawlers", () => {
  assert.equal(parseAgent("Mozilla/5.0 (Macintosh) Safari/605"), null);
  assert.equal(parseAgent("curl/8.4.0"), null);
  assert.equal(parseAgent(""), null);
  assert.equal(parseAgent(null), null);
});

test("caps field lengths so a hostile agent cannot bloat the table", () => {
  const parsed = parseAgent("NotesAnkify/" + "9".repeat(100) + " (darwin; arm64)");
  assert.equal(parsed.version.length, 32);
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `node --test cloudflare/`
Expected: FAIL, cannot find module `./worker.js`.

- [ ] **Step 3: Write worker.js**

```js
// Cloudflare Worker on the route notesankify.com/version.json.
//
// It proxies the file from GitHub Pages unchanged and, after the response
// is on its way, records one row per install per day in D1. Only the
// app's own User-Agents are recorded. Nothing that identifies a person
// is stored: the visitor hash mixes the day, the client IP, the
// User-Agent and a secret salt, and cannot be reversed or joined across
// days.

const AGENT_APP = /^NotesAnkify\/(\S{1,32}) \((\w{1,16}); (\w{1,16})\)/;
const AGENT_LEGACY = /^Go-http-client\//;

export function parseAgent(ua) {
  if (!ua) return null;
  const m = AGENT_APP.exec(ua);
  if (m) return { client: "app", version: m[1], os: m[2], arch: m[3] };
  if (AGENT_LEGACY.test(ua)) return { client: "legacy", version: "legacy", os: "unknown", arch: "unknown" };
  return null;
}

async function sha256Hex(text) {
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(text));
  return [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, "0")).join("");
}

async function record(request, env) {
  const ua = request.headers.get("User-Agent") || "";
  const parsed = parseAgent(ua);
  if (!parsed) return;
  const day = new Date().toISOString().slice(0, 10);
  const ip = request.headers.get("CF-Connecting-IP") || "";
  const visitor = (await sha256Hex(`${day}|${ip}|${ua}|${env.SALT}`)).slice(0, 32);
  await env.DB.prepare(
    "INSERT OR IGNORE INTO pings (day, visitor, os, arch, version, client) VALUES (?, ?, ?, ?, ?, ?)"
  ).bind(day, visitor, parsed.os, parsed.arch, parsed.version, parsed.client).run();
}

export default {
  async fetch(request, env, ctx) {
    try {
      const response = await fetch(request, { cf: { cacheTtl: 300, cacheEverything: true } });
      ctx.waitUntil(record(request, env).catch(() => {}));
      return response;
    } catch (_) {
      // Never let a Worker problem break update checks.
      return fetch(request);
    }
  },
};
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `node --test cloudflare/`
Expected: 4 passing.

- [ ] **Step 5: Commit**

```bash
git add cloudflare/worker.js cloudflare/worker.test.mjs
git commit -s -m "feat(metrics): count daily version checks at the edge without identifying anyone"
```

### Task 20: Wrangler config, schema, queries and setup guide

**Files:**
- Create: `cloudflare/wrangler.toml`
- Create: `cloudflare/schema.sql`
- Create: `cloudflare/stats.sql`
- Create: `cloudflare/README.md`

- [ ] **Step 1: wrangler.toml**

```toml
name = "notesankify-version-ping"
main = "worker.js"
compatibility_date = "2026-09-01"

routes = [
  { pattern = "notesankify.com/version.json", zone_name = "notesankify.com" }
]

[[d1_databases]]
binding = "DB"
database_name = "notesankify-pings"
# Paste the id printed by: wrangler d1 create notesankify-pings
database_id = ""
```

- [ ] **Step 2: schema.sql**

```sql
CREATE TABLE IF NOT EXISTS pings (
  day     TEXT NOT NULL,  -- YYYY-MM-DD, UTC
  visitor TEXT NOT NULL,  -- first 32 hex chars of sha256(day|ip|user_agent|salt)
  os      TEXT NOT NULL,  -- darwin, windows, linux, unknown
  arch    TEXT NOT NULL,  -- arm64, amd64, unknown
  version TEXT NOT NULL,  -- app version, or "legacy" for 1.0.0 clients
  client  TEXT NOT NULL,  -- "app" or "legacy"
  PRIMARY KEY (day, visitor)
);

CREATE INDEX IF NOT EXISTS pings_day ON pings (day);
```

- [ ] **Step 3: stats.sql**

```sql
-- Daily active installs, last 30 days
SELECT day, COUNT(*) AS installs
FROM pings
WHERE day >= date('now', '-30 days')
GROUP BY day
ORDER BY day DESC;

-- Weekly active installs, last 12 weeks (a visitor hash is per day, so this
-- counts install-days; divide by 7 for an average daily figure)
SELECT strftime('%Y-W%W', day) AS week, COUNT(*) AS install_days
FROM pings
WHERE day >= date('now', '-84 days')
GROUP BY week
ORDER BY week DESC;

-- By operating system, last 7 days
SELECT os, COUNT(*) AS install_days
FROM pings
WHERE day >= date('now', '-7 days')
GROUP BY os
ORDER BY install_days DESC;

-- By app version, last 7 days (watch a new release take over here)
SELECT version, COUNT(*) AS install_days
FROM pings
WHERE day >= date('now', '-7 days')
GROUP BY version
ORDER BY install_days DESC;
```

- [ ] **Step 4: README.md**

```markdown
# Usage counting for notesankify.com

The app fetches `https://notesankify.com/version.json` once a day. This
Worker sits on that route, proxies the file from GitHub Pages, and counts
one row per install per day in a D1 database. No identifier is stored:
the visitor hash cannot be reversed or joined across days.

## One-time setup

1. **Cloudflare account and zone.** Add `notesankify.com` to a free
   Cloudflare account. It imports the existing records: the four GitHub
   Pages A records on the apex and the `www` CNAME. Set each to Proxied.
   Under SSL/TLS choose **Full** (not Full strict: GitHub Pages renews
   its own certificate behind the proxy).

2. **Nameservers.** At Namecheap, Domain List, Manage, Nameservers, choose
   Custom DNS and enter the two nameservers Cloudflare shows. Records do
   not change, so the site stays up while this propagates. Verify with:

       dig +short NS notesankify.com
       curl -sI https://notesankify.com/version.json | grep -i '^server'

   The second line says `cloudflare` when the proxy is active.

3. **Worker and database**, from this folder:

       npm install -g wrangler
       wrangler login
       wrangler d1 create notesankify-pings

   Paste the printed `database_id` into `wrangler.toml`. Then:

       wrangler d1 execute notesankify-pings --remote --file schema.sql
       wrangler secret put SALT          # paste a long random string, keep it in the password manager
       wrangler deploy

4. **Check it works.** Request the file once with the app's User-Agent and
   look for the row:

       curl -s -A "NotesAnkify/1.1.0 (darwin; arm64)" https://notesankify.com/version.json > /dev/null
       wrangler d1 execute notesankify-pings --remote --command "SELECT * FROM pings ORDER BY day DESC LIMIT 5"

## Reading the numbers

    wrangler d1 execute notesankify-pings --remote --file stats.sql

The first query is daily active installs. Installs still on 1.0.0 show as
version `legacy`, because that build sends Go's default User-Agent.

## Changing the Worker

Edit `worker.js`, run `node --test .`, then `wrangler deploy`. If the
Worker ever throws, it falls back to a plain proxy, so update checks keep
working.

## Free-tier fit

Expected traffic is on the order of ten thousand requests a day. The free
plan allows one hundred thousand Worker requests and one hundred thousand
D1 row writes a day.
```

- [ ] **Step 5: Validate and commit**

Run: `grep -c 'pattern = "notesankify.com/version.json"' cloudflare/wrangler.toml && node --test cloudflare/`
Expected: `1` and the four Worker tests still passing. `wrangler deploy` validates the TOML fully when the maintainer deploys.

```bash
git add cloudflare/wrangler.toml cloudflare/schema.sql cloudflare/stats.sql cloudflare/README.md
git commit -s -m "feat(metrics): wrangler config, D1 schema, queries and setup guide"
```

### Task 21: Deploy and the privacy sentence

**Files:**
- Modify (website repo `kpauljoseph/notesankify.github.io`): `privacy/index.html`

This task needs the maintainer's Cloudflare account and the DNS change, so it is executed by the maintainer following `cloudflare/README.md`; the engineer prepares the website edit.

- [ ] **Step 1: Add the privacy sentence**

In the website repo, in `privacy/index.html`, inside the "Information We Collect" section, add a paragraph:

```html
<p>The NotesAnkify app checks notesankify.com once a day for updates. That request carries the app version and operating system. We count these requests to estimate how many people use NotesAnkify. No identifier is stored and requests cannot be linked to a person or across days.</p>
```

Commit there with `docs: describe the daily update check` and push.

- [ ] **Step 2: Deploy the Worker**

Follow `cloudflare/README.md` steps 1 to 4. Expected at the end: the `SELECT` shows one row with `client = app`.

- [ ] **Step 3: Confirm existing installs are being counted**

The next day:

    wrangler d1 execute notesankify-pings --remote --command "SELECT client, COUNT(*) FROM pings WHERE day = date('now', '-1 day') GROUP BY client"

Expected: a `legacy` row in the hundreds or more, from 1.0.0 installs that already call version.json daily.
