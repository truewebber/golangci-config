package remote

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type HTTPFetcher struct {
	client   *http.Client
	cacheDir string
}

func NewHTTPFetcher(cacheDir string, timeout time.Duration) *HTTPFetcher {
	return &HTTPFetcher{
		client:   &http.Client{Timeout: timeout},
		cacheDir: cacheDir,
	}
}

const (
	writePerm   = 0o600
	makeDirPerm = 0o750
)

var errUnexpectedHTTPStatus = errors.New("unexpected HTTP status")

func (f *HTTPFetcher) Fetch(url string) (data []byte, fromCache bool, retErr error) {
	cachePath, etagPath, cacheErr := f.cachePaths(url)
	if cacheErr != nil {
		return nil, false, fmt.Errorf("cache paths: %w", cacheErr)
	}

	req, reqErr := http.NewRequest(http.MethodGet, url, nil)
	if reqErr != nil {
		return nil, false, fmt.Errorf("new http request: %w", reqErr)
	}

	if etag, err := os.ReadFile(etagPath); err == nil {
		if trimmed := strings.TrimSpace(string(etag)); trimmed != "" {
			req.Header.Set("If-None-Match", trimmed)
		}
	}

	resp, doErr := f.client.Do(req)
	if doErr != nil {
		body, readErr := os.ReadFile(cachePath)
		if readErr == nil {
			return body, true, nil
		}

		return nil, false, fmt.Errorf("fetch %s: %w", url, doErr)
	}

	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			if fallback, fileErr := os.ReadFile(cachePath); fileErr == nil {
				return fallback, true, nil
			}

			return nil, false, fmt.Errorf("fetch %s: %w", url, readErr)
		}

		if err := f.ensureCacheDir(); err == nil {
			_ = os.WriteFile(cachePath, body, writePerm)
			if newETag := strings.TrimSpace(resp.Header.Get("ETag")); newETag != "" {
				_ = os.WriteFile(etagPath, []byte(newETag), writePerm)
			}
		}

		return body, false, nil
	case http.StatusNotModified:
		body, err := os.ReadFile(cachePath)
		if err != nil {
			return nil, false, fmt.Errorf("remote responded 304 but cache unavailable: %w", err)
		}

		return body, true, nil
	default:
		body, readErr := os.ReadFile(cachePath)
		if readErr == nil {
			return body, true, nil
		}

		return nil, false, fmt.Errorf("%w: %d", errUnexpectedHTTPStatus, resp.StatusCode)
	}
}

func (f *HTTPFetcher) ensureCacheDir() error {
	if err := os.MkdirAll(f.cacheDir, makeDirPerm); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	return nil
}

var errCacheDirectoryIsEmpty = errors.New("cache directory is empty")

func (f *HTTPFetcher) cachePaths(url string) (cachePath, etagPath string, err error) {
	if strings.TrimSpace(f.cacheDir) == "" {
		return "", "", errCacheDirectoryIsEmpty
	}

	hash := sha256.Sum256([]byte(url))
	name := hex.EncodeToString(hash[:])
	cachePath = filepath.Join(f.cacheDir, name+".yml")
	etagPath = filepath.Join(f.cacheDir, name+".etag")

	return cachePath, etagPath, nil
}
