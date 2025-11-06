package configinfra

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	domainconfig "github.com/truewebber/golangci-config/internal/domain/config"
	"github.com/truewebber/golangci-config/internal/log"
)

type RemoteFetcher interface {
	Fetch(url string) (data []byte, fromCache bool, err error)
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

	if localDocument == nil {
		localDocument = map[string]interface{}{}
	}

	remoteURL := domainconfig.ExtractRemoteURL(data)
	merged := localDocument

	if remoteURL != "" {
		s.logger.Info("Remote configuration directive found", "url", remoteURL)

		remoteData, fromCache, fetchErr := s.fetcher.Fetch(remoteURL)
		if fetchErr != nil {
			s.logger.Warn("Unable to fetch remote configuration; using local config only", "error", fetchErr)
		} else if len(remoteData) == 0 {
			s.logger.Warn("Remote configuration is empty; using local config only")
		} else {
			remoteDocument, err := domainconfig.NormalizeYAML(remoteData)
			if err != nil {
				s.logger.Warn("Failed to parse remote configuration; using local config only", "error", err)
			} else if remoteDocument != nil {
				if fromCache {
					s.logger.Warn("Using cached remote configuration", "url", remoteURL)
				}

				merged = domainconfig.Merge(remoteDocument, localDocument)
			}
		}
	} else {
		s.logger.Warn("Remote configuration directive not found. Using local configuration only.",
			"directive", domainconfig.RemoteDirective)
	}

	generatedPath := domainconfig.GeneratedPath(localConfigPath)
	if err := s.cleanupGeneratedFiles(generatedPath); err != nil {
		return "", fmt.Errorf("cleanup generated files: %w", err)
	}

	yamlBytes, err := yamlMarshal(merged)
	if err != nil {
		return "", fmt.Errorf("yaml marshal: %w", err)
	}

	header := domainconfig.Header(remoteURL, localConfigPath)
	if err := writeFileAtomic(generatedPath, header, yamlBytes); err != nil {
		return "", fmt.Errorf("write file atomic: %w", err)
	}

	s.logger.Info("Generated configuration file", "path", generatedPath)

	return generatedPath, nil
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

		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("os remove: %w", err)
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
