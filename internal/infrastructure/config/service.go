package configinfra

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	domainconfig "github.com/truewebber/golangci-config/internal/domain/config"
	"github.com/truewebber/golangci-config/internal/log"
)

//go:generate go run go.uber.org/mock/mockgen -source=service.go -destination=../remote/mock.go -package remote
type RemoteFetcher interface {
	Fetch(u *url.URL) (data []byte, fromCache bool, err error)
}

type Service struct {
	logger  log.Logger
	fetcher RemoteFetcher
}

func NewService(logger log.Logger, fetcher RemoteFetcher) *Service {
	return &Service{
		logger:  logger,
		fetcher: fetcher,
	}
}

func (s *Service) Prepare(localConfigPath string) (string, error) {
	data, err := os.ReadFile(localConfigPath)
	if err != nil {
		return "", fmt.Errorf("read local configuration %s: %w", localConfigPath, err)
	}

	localDocument, err := domainconfig.NormalizeYAML(data)
	if err != nil {
		return "", fmt.Errorf("parse local configuration %s: %w", localConfigPath, err)
	}

	remoteURL, err := domainconfig.ExtractRemoteURL(data)
	if err != nil {
		s.logger.Warn("failed to extract remote URL from local configuration", "error", err)
	}

	var remoteDocument interface{}

	if remoteURL != nil {
		var rcErr error

		remoteDocument, rcErr = s.remoteConfigContents(remoteURL)
		if rcErr != nil {
			s.logger.Warn("Failed to extract remote document from local configuration", "error", rcErr)
		}
	}

	if remoteDocument == nil {
		s.logger.Warn("Remote configuration directive not found. Using local configuration only")
	}

	merged := domainconfig.Merge(remoteDocument, localDocument)

	generatedPath := domainconfig.GeneratedPath(localConfigPath)
	if cleanupErr := s.cleanupGeneratedFiles(generatedPath); cleanupErr != nil {
		return "", fmt.Errorf("cleanup generated files: %w", cleanupErr)
	}

	yamlBytes, err := yamlMarshal(merged)
	if err != nil {
		return "", fmt.Errorf("yaml marshal: %w", err)
	}

	header := domainconfig.Header(remoteURL, localConfigPath)
	if writeErr := writeFileAtomic(generatedPath, header, yamlBytes); writeErr != nil {
		return "", fmt.Errorf("write file atomic: %w", writeErr)
	}

	s.logger.Info("Generated configuration file", "path", generatedPath)

	return generatedPath, nil
}

func (s *Service) remoteConfigContents(remoteURL *url.URL) (interface{}, error) {
	remoteData, fromCache, err := s.fetcher.Fetch(remoteURL)
	if err != nil {
		return nil, fmt.Errorf("fetch remote configuration: %w", err)
	}

	if fromCache {
		s.logger.Warn("Using cached remote configuration")
	}

	remoteDocument, err := domainconfig.NormalizeYAML(remoteData)
	if err != nil {
		return nil, fmt.Errorf("parse remote configuration: %w", err)
	}

	return remoteDocument, nil
}

func (s *Service) cleanupGeneratedFiles(current string) error {
	absCurrent, filepathErr := filepath.Abs(current)
	if filepathErr != nil {
		return fmt.Errorf("resolve generated config path: %w", filepathErr)
	}

	if err := filepath.WalkDir(".", s.walkThrough(absCurrent)); err != nil {
		return fmt.Errorf("walk dir: %w", err)
	}

	return nil
}

func (s *Service) walkThrough(absCurrent string) func(path string, d os.DirEntry, walkErr error) error {
	return func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk dir: %w", walkErr)
		}

		if d.IsDir() {
			return nil
		}

		if filepath.Base(path) != domainconfig.GeneratedFileName {
			return nil
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("absolute path: %w", err)
		}

		if absPath == absCurrent {
			return nil
		}

		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("os remove: %w", removeErr)
		}

		s.logger.Info("Removed old generated config", "path", path)

		return nil
	}
}

func yamlMarshal(value interface{}) ([]byte, error) {
	data, err := yaml.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode merged configuration: %w", err)
	}

	return data, nil
}

const writePerm = 0o600

func writeFileAtomic(path, header string, body []byte) error {
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, append([]byte(header), body...), writePerm); err != nil {
		return fmt.Errorf("write generated configuration: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("finalize generated configuration: %w", err)
	}

	return nil
}
