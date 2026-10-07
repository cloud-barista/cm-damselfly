// Package modelschema loads the Go struct definitions of a tagged model module (e.g. cm-beetle/imdl)
// at runtime and validates/normalizes JSON documents against them, without compiling that version in.
package modelschema

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Source describes where the tagged source code of a model module can be downloaded.
type Source struct {
	// ModulePath is the Go module path (e.g. "github.com/cloud-barista/cm-beetle/imdl").
	ModulePath string
	// Owner, Repo, RepoSubdir and TagPrefix locate the module in its GitHub repository
	// (e.g. "cloud-barista", "cm-beetle", "imdl", "imdl/"). Used as a fallback source.
	Owner      string
	Repo       string
	RepoSubdir string
	TagPrefix  string
}

const maxDownloadSize = 20 << 20

var (
	httpClient = &http.Client{Timeout: 30 * time.Second}

	packageCacheMu sync.Mutex
	packageCache   = map[string]*Package{}
)

// LoadPackage returns the parsed Go package 'pkgDir' (relative to the module root; empty for the root package) of the given tagged version.
// Tagged versions are immutable, so successfully parsed packages are cached for the lifetime of the process.
func LoadPackage(ctx context.Context, src Source, version, pkgDir string) (*Package, error) {
	cacheKey := src.ModulePath + "@" + version + "/" + pkgDir

	packageCacheMu.Lock()
	cached, ok := packageCache[cacheKey]
	packageCacheMu.Unlock()
	if ok {
		return cached, nil
	}

	files, proxyErr := fetchFromModuleProxy(ctx, src, version, pkgDir)
	if proxyErr != nil {
		var githubErr error
		files, githubErr = fetchFromGitHub(ctx, src, version, pkgDir)
		if githubErr != nil {
			return nil, fmt.Errorf("failed to download %s@%s/%s (module proxy: %v; GitHub: %v)", src.ModulePath, version, pkgDir, proxyErr, githubErr)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no Go source files found in %s@%s/%s", src.ModulePath, version, pkgDir)
	}

	pkg, err := ParsePackage(files)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s@%s/%s: %w", src.ModulePath, version, pkgDir, err)
	}

	packageCacheMu.Lock()
	packageCache[cacheKey] = pkg
	packageCacheMu.Unlock()
	return pkg, nil
}

func moduleProxyURLs() []string {
	var urls []string
	for _, entry := range strings.FieldsFunc(os.Getenv("GOPROXY"), func(r rune) bool { return r == ',' || r == '|' }) {
		entry = strings.TrimSpace(entry)
		if strings.HasPrefix(entry, "https://") || strings.HasPrefix(entry, "http://") {
			urls = append(urls, strings.TrimSuffix(entry, "/"))
		}
	}
	if len(urls) == 0 {
		urls = append(urls, "https://proxy.golang.org")
	}
	return urls
}

// escapeModulePath applies the Go module proxy case-encoding ('A' -> '!a').
func escapeModulePath(p string) string {
	var b strings.Builder
	for _, r := range p {
		if unicode.IsUpper(r) {
			b.WriteByte('!')
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func fetchFromModuleProxy(ctx context.Context, src Source, version, pkgDir string) (map[string][]byte, error) {
	var lastErr error
	for _, proxyURL := range moduleProxyURLs() {
		url := fmt.Sprintf("%s/%s/@v/%s.zip", proxyURL, escapeModulePath(src.ModulePath), escapeModulePath(version))
		data, err := download(ctx, url, nil)
		if err != nil {
			lastErr = err
			continue
		}

		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			lastErr = err
			continue
		}

		dirPrefix := src.ModulePath + "@" + version + "/"
		if dir := strings.Trim(pkgDir, "/"); dir != "" {
			dirPrefix += dir + "/"
		}
		files := map[string][]byte{}
		for _, f := range zr.File {
			if !strings.HasPrefix(f.Name, dirPrefix) || !isPackageSourceFile(strings.TrimPrefix(f.Name, dirPrefix)) {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			content, err := io.ReadAll(io.LimitReader(rc, maxDownloadSize))
			rc.Close()
			if err != nil {
				return nil, err
			}
			files[path.Base(f.Name)] = content
		}
		return files, nil
	}
	return nil, lastErr
}

func fetchFromGitHub(ctx context.Context, src Source, version, pkgDir string) (map[string][]byte, error) {
	if src.Owner == "" || src.Repo == "" {
		return nil, fmt.Errorf("GitHub repository is not specified")
	}

	dir := strings.Trim(path.Join(src.RepoSubdir, pkgDir), "/")
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s%s", src.Owner, src.Repo, dir, src.TagPrefix, version)
	data, err := download(ctx, url, map[string]string{"Accept": "application/vnd.github+json"})
	if err != nil {
		return nil, err
	}

	var entries []struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		DownloadURL string `json:"download_url"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}

	files := map[string][]byte{}
	for _, entry := range entries {
		if entry.Type != "file" || !isPackageSourceFile(entry.Name) || entry.DownloadURL == "" {
			continue
		}
		content, err := download(ctx, entry.DownloadURL, nil)
		if err != nil {
			return nil, err
		}
		files[entry.Name] = content
	}
	return files, nil
}

func isPackageSourceFile(name string) bool {
	return !strings.Contains(name, "/") && strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

func download(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP status %d from %s", resp.StatusCode, url)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownloadSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxDownloadSize {
		return nil, fmt.Errorf("response from %s exceeds %d bytes", url, maxDownloadSize)
	}
	return data, nil
}
