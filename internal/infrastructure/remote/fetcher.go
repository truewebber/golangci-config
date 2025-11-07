package remote

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/truewebber/golangci-config/internal/log"
)

type HTTPFetcher struct {
	logger   log.Logger
	client   *http.Client
	cacheDir string
}

func NewHTTPFetcher(
	logger log.Logger,
	cacheDir string,
	timeout time.Duration,
) *HTTPFetcher {
	return &HTTPFetcher{
		logger:   logger,
		client:   &http.Client{Timeout: timeout},
		cacheDir: cacheDir,
	}
}

const (
	writePerm   = 0o600
	makeDirPerm = 0o750
)

var errUnexpectedHTTPStatus = errors.New("unexpected HTTP status")

func (f *HTTPFetcher) Fetch(u *url.URL) (data []byte, fromCache bool, retErr error) {
	cachePath, etagPath, cacheErr := f.cachePaths(u)
	if cacheErr != nil {
		return nil, false, fmt.Errorf("cache paths: %w", cacheErr)
	}

	resp, fetchErr := f.fetchFromRemote(u, etagPath)

	if fetchErr != nil {
		f.logger.Warn("Failed to fetch from remote", "url", u, "err", fetchErr)
	}

	if fetchErr != nil || resp.notModified {
		body, readErr := os.ReadFile(cachePath)
		if readErr != nil {
			return nil, false, fmt.Errorf("read cache file: %w", readErr)
		}

		return body, true, nil
	}

	if err := f.writeNewCache(cachePath, etagPath, resp.body, resp.etag); err != nil {
		f.logger.Warn("Failed to write new cache",
			"cache_path", cachePath,
			"etag_path", etagPath,
			"err", err,
		)
	}

	return resp.body, false, nil
}

type responseBody struct {
	body        []byte
	etag        string
	notModified bool
}

func (f *HTTPFetcher) fetchFromRemote(u *url.URL, etagPath string) (responseBody, error) {
	req, reqErr := http.NewRequest(http.MethodGet, u.String(), http.NoBody)
	if reqErr != nil {
		return responseBody{}, fmt.Errorf("new http request: %w", reqErr)
	}

	f.setEtagHeader(req, etagPath)

	resp, doErr := f.client.Do(req)
	if doErr != nil {
		return responseBody{}, fmt.Errorf("do request: %w", doErr)
	}

	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return responseBody{}, fmt.Errorf("read all: %w", readErr)
		}

		return responseBody{
			body:        body,
			etag:        strings.TrimSpace(resp.Header.Get("ETag")),
			notModified: false,
		}, nil
	case http.StatusNotModified:
		return responseBody{
			body:        nil,
			etag:        "",
			notModified: true,
		}, nil
	default:
		return responseBody{}, fmt.Errorf("%w: %d", errUnexpectedHTTPStatus, resp.StatusCode)
	}
}

func (f *HTTPFetcher) setEtagHeader(req *http.Request, etagPath string) {
	etag, err := os.ReadFile(etagPath)
	if err != nil {
		return
	}

	if trimmed := strings.TrimSpace(string(etag)); trimmed != "" {
		req.Header.Set("If-None-Match", trimmed)
	}
}

func (f *HTTPFetcher) writeNewCache(
	cachePath, etagPath string,
	body []byte,
	etag string,
) error {
	ensureErr := f.ensureCacheDir()
	if ensureErr != nil {
		return fmt.Errorf("ensure cache dir: %w", ensureErr)
	}

	if err := os.WriteFile(cachePath, body, writePerm); err != nil {
		return fmt.Errorf("write cache file: %w", err)
	}

	if err := os.WriteFile(etagPath, []byte(etag), writePerm); err != nil {
		return fmt.Errorf("write etag file: %w", err)
	}

	return nil
}

func (f *HTTPFetcher) ensureCacheDir() error {
	if err := os.MkdirAll(f.cacheDir, makeDirPerm); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	return nil
}

var errCacheDirectoryIsEmpty = errors.New("cache directory is empty")

func (f *HTTPFetcher) cachePaths(u *url.URL) (cachePath, etagPath string, err error) {
	if strings.TrimSpace(f.cacheDir) == "" {
		return "", "", errCacheDirectoryIsEmpty
	}

	hash := sha256.Sum256([]byte(u.String()))
	name := hex.EncodeToString(hash[:])
	cachePath = filepath.Join(f.cacheDir, name+".yml")
	etagPath = filepath.Join(f.cacheDir, name+".etag")

	return cachePath, etagPath, nil
}
