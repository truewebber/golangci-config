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

func (f *HTTPFetcher) Fetch(url string) (data []byte, fromCache bool, err error) {
	cachePath, etagPath, err := f.cachePaths(url)
	if err != nil {
		return nil, false, err
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, false, fmt.Errorf("create request: %w", err)
	}

	if etag, err := os.ReadFile(etagPath); err == nil {
		if trimmed := strings.TrimSpace(string(etag)); trimmed != "" {
			req.Header.Set("If-None-Match", trimmed)
		}
	}

	resp, err := f.client.Do(req)
	if err != nil {
		body, cacheErr := os.ReadFile(cachePath)
		if cacheErr == nil {
			return body, true, nil
		}
		return nil, false, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			if fallback, cacheErr := os.ReadFile(cachePath); cacheErr == nil {
				return fallback, true, nil
			}
			return nil, false, err
		}
		if err := f.ensureCacheDir(); err == nil {
			_ = os.WriteFile(cachePath, body, 0o644)
			if newETag := strings.TrimSpace(resp.Header.Get("ETag")); newETag != "" {
				_ = os.WriteFile(etagPath, []byte(newETag), 0o644)
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
		body, cacheErr := os.ReadFile(cachePath)
		if cacheErr == nil {
			return body, true, nil
		}
		return nil, false, fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}
}

func (f *HTTPFetcher) ensureCacheDir() error {
	return os.MkdirAll(f.cacheDir, 0o755)
}

func (f *HTTPFetcher) cachePaths(url string) (cachePath, etagPath string, err error) {
	if strings.TrimSpace(f.cacheDir) == "" {
		return "", "", errors.New("cache directory is empty")
	}
	hash := sha256.Sum256([]byte(url))
	name := hex.EncodeToString(hash[:])
	cachePath = filepath.Join(f.cacheDir, name+".yml")
	etagPath = filepath.Join(f.cacheDir, name+".etag")
	return cachePath, etagPath, nil
}
