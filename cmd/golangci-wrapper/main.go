package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	generatedFileName         = ".golangci.generated.yml"
	remoteDirective           = "GOLANGCI_LINT_REMOTE_CONFIG"
	defaultHTTPTimeout        = 15 * time.Second
	cacheDirectoryName        = ".cache/golangci-wrapper"
	directivePattern          = `(?i)GOLANGCI_LINT_REMOTE_CONFIG:\s*(\S+)`
	localConfigPrimaryYAML    = ".golangci.local.yml"
	localConfigPrimaryYAMLAlt = ".golangci.local.yaml"
	localConfigDefaultYAML    = ".golangci.yml"
	localConfigDefaultYAMLAlt = ".golangci.yaml"
)

var directiveRegex = regexp.MustCompile(directivePattern)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("[ERROR] %v\n", err)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printUsage()

		return nil
	}

	localConfig, err := findLocalConfig(args)
	if err != nil {
		return err
	}

	var mergedConfigPath string

	if localConfig != "" {
		mergedConfigPath, err = prepareMergedConfig(localConfig)
		if err != nil {
			return err
		}
	} else {
		log.Println("[WARN] Local configuration file not found; running golangci-lint without generated config")
	}

	if err := ensureGolangciLintAvailable(); err != nil {
		return fmt.Errorf("ensure golangci-lint available: %w", err)
	}

	finalArgs := buildFinalArgs(args, mergedConfigPath, localConfig)

	return runGolangciLint(finalArgs)
}

func printUsage() {
	log.Println("Usage: golangci-wrapper run [golangci-lint flags]")
	log.Println()
	log.Println("The wrapper looks for a local configuration file (.golangci.local.yml/.yaml or .golangci.yml/.yaml).")
	log.Println("If the file contains a directive in comments of the form:")
	log.Println("  // GOLANGCI_LINT_REMOTE_CONFIG: https://example.com/config.yml")
	log.Println("the remote configuration is downloaded, merged with the local one, and passed to golangci-lint.")
	log.Println("Without the directive the wrapper uses only the local configuration.")
	log.Println()
	log.Println("Examples:")
	log.Println("  golangci-wrapper run")
	log.Println("  golangci-wrapper run ./...")
	log.Println("  golangci-wrapper run -c custom.yml ./...")
	log.Println()
	log.Println("Make sure golangci-lint is installed, e.g.:")
	log.Println("  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest")
}

func ensureGolangciLintAvailable() error {
	cmd := exec.Command("go", "tool", "-n", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("golangci-lint via go tool is unavailable: %w", err)
	}

	return nil
}

func findLocalConfig(args []string) (string, error) {
	if cfg, provided := configFromArgs(args); provided {
		if cfg == "" {
			return "", errors.New("missing value for -c/--config")
		}
		return cfg, nil
	}

	candidates := []string{
		localConfigPrimaryYAML,
		localConfigPrimaryYAMLAlt,
		localConfigDefaultYAML,
		localConfigDefaultYAMLAlt,
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", nil
}

func configFromArgs(args []string) (string, bool) {
	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == "-c", arg == "--config":
			if i+1 < len(args) {
				return args[i+1], true
			}
			return "", true
		case strings.HasPrefix(arg, "--config="):
			return strings.TrimPrefix(arg, "--config="), true
		}
	}

	return "", false
}

func prepareMergedConfig(localConfig string) (string, error) {
	data, err := os.ReadFile(localConfig)
	if err != nil {
		return "", fmt.Errorf("read local configuration %s: %w", localConfig, err)
	}

	localDocument, err := readYAMLBytes(data)
	if err != nil {
		return "", fmt.Errorf("parse local configuration %s: %w", localConfig, err)
	}

	if localDocument == nil {
		localDocument = map[string]interface{}{}
	}

	remoteURL := extractRemoteURL(data)
	var merged = localDocument

	if remoteURL != "" {
		log.Printf("[INFO] Remote configuration directive found: %s\n", remoteURL)

		remoteDoc, fetchErr := fetchRemoteConfig(remoteURL)
		if fetchErr != nil {
			log.Printf("[WARN] Unable to fetch remote configuration (%v); using local config only\n", fetchErr)
		} else {
			if remoteDoc == nil {
				log.Printf("[WARN] Remote configuration is empty; using local config only\n")
			} else {
				merged = mergeYAMLDocuments(remoteDoc, localDocument)
			}
		}
	} else {
		log.Printf("[WARN] Remote configuration directive (%s) not found. Using local configuration only.\n",
			remoteDirective)
	}

	generatedPath := generatedConfigPath(localConfig)
	if err := writeGeneratedConfig(generatedPath, merged, remoteURL, localConfig); err != nil {
		return "", err
	}

	return generatedPath, nil
}

func extractRemoteURL(data []byte) string {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			matches := directiveRegex.FindStringSubmatch(line)
			if len(matches) == 2 {
				return matches[1]
			}
		}
	}

	return ""
}

func fetchRemoteConfig(url string) (interface{}, error) {
	body, err := downloadRemoteConfig(url)
	if err != nil {
		return nil, fmt.Errorf("download remote config: %w", err)
	}

	result, err := readYAMLBytes(body)
	if err != nil {
		return nil, fmt.Errorf("parse remote configuration: %w", err)
	}

	return result, nil
}

func downloadRemoteConfig(url string) ([]byte, error) {
	cacheBase, err := cacheBasePathForURL(url)
	if err != nil {
		return nil, fmt.Errorf("cache base path for URL: %w", err)
	}

	cachePath := cacheBase + ".yml"
	etagPath := cacheBase + ".etag"

	client := &http.Client{Timeout: defaultHTTPTimeout}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("new http request: %w", err)
	}

	if etag, err := os.ReadFile(etagPath); err == nil {
		if trimmed := strings.TrimSpace(string(etag)); trimmed != "" {
			req.Header.Set("If-None-Match", trimmed)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return useCachedOrError(cachePath, err)
	}

	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return useCachedOrError(cachePath, err)
		}

		if err := ensureCacheDir(); err == nil {
			_ = os.WriteFile(cachePath, body, 0o644)
			if newETag := strings.TrimSpace(resp.Header.Get("ETag")); newETag != "" {
				_ = os.WriteFile(etagPath, []byte(newETag), 0o644)
			}
		}

		return body, nil
	case http.StatusNotModified:
		body, err := os.ReadFile(cachePath)
		if err != nil {
			return nil, fmt.Errorf("remote responded 304 but cache is unavailable: %w", err)
		}

		return body, nil
	default:
		return useCachedOrError(cachePath, fmt.Errorf("unexpected HTTP status %d", resp.StatusCode))
	}
}

func useCachedOrError(cachePath string, originalErr error) ([]byte, error) {
	if cachePath != "" {
		if body, err := os.ReadFile(cachePath); err == nil {
			log.Printf("[WARN] Falling back to cached remote configuration: %s\n", cachePath)

			return body, nil
		}
	}

	return nil, originalErr
}

func ensureCacheDir() error {
	dir, err := cacheDir()
	if err != nil {
		return fmt.Errorf("get cache dir: %w", err)
	}

	return os.MkdirAll(dir, 0o755)
}

func cacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}

	return filepath.Join(home, cacheDirectoryName), nil
}

func cacheBasePathForURL(url string) (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", fmt.Errorf("get cache dir: %w", err)
	}

	hash := sha256.Sum256([]byte(url))
	name := hex.EncodeToString(hash[:])

	return filepath.Join(dir, name), nil
}

func readYAMLBytes(data []byte) (interface{}, error) {
	var content interface{}
	if err := yaml.Unmarshal(data, &content); err != nil {
		return nil, fmt.Errorf("unmarshal YAML: %w", err)
	}

	return normalizeYAML(content), nil
}

func normalizeYAML(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))

		for key, val := range v {
			result[key] = normalizeYAML(val)
		}

		return result
	case map[interface{}]interface{}:
		result := make(map[string]interface{}, len(v))

		for key, val := range v {
			result[fmt.Sprint(key)] = normalizeYAML(val)
		}

		return result
	case []interface{}:
		result := make([]interface{}, len(v))

		for i, val := range v {
			result[i] = normalizeYAML(val)
		}

		return result
	default:
		return v
	}
}

func mergeYAMLDocuments(base, override interface{}) interface{} {
	switch baseTyped := base.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(baseTyped))

		for key, val := range baseTyped {
			result[key] = deepCopy(val)
		}

		overrideMap, ok := override.(map[string]interface{})
		if !ok {
			return deepCopy(override)
		}

		for key, overrideVal := range overrideMap {
			if existing, ok := result[key]; ok {
				result[key] = mergeYAMLDocuments(existing, overrideVal)
			} else {
				result[key] = deepCopy(overrideVal)
			}
		}

		return result
	case []interface{}:
		overrideSlice, ok := override.([]interface{})
		if !ok {
			return deepCopy(override)
		}

		return deepCopy(overrideSlice)
	default:
		return deepCopy(override)
	}
}

func deepCopy(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))
		for key, val := range v {
			result[key] = deepCopy(val)
		}

		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = deepCopy(item)
		}

		return result
	default:
		return v
	}
}

func generatedConfigPath(localConfig string) string {
	dir := filepath.Dir(localConfig)
	if dir == "." {
		return generatedFileName
	}

	return filepath.Join(dir, generatedFileName)
}

func writeGeneratedConfig(path string, content interface{}, remoteURL, localConfig string) error {
	if err := cleanupOldGeneratedConfigs(path); err != nil {
		return err
	}

	yamlBytes, err := yaml.Marshal(content)
	if err != nil {
		return fmt.Errorf("encode merged configuration: %w", err)
	}

	header := generatedHeader(remoteURL, localConfig)
	tempFile := path + ".tmp"

	if err := os.WriteFile(tempFile, append([]byte(header), yamlBytes...), 0o644); err != nil {
		return fmt.Errorf("write generated configuration: %w", err)
	}

	if err := os.Rename(tempFile, path); err != nil {
		return fmt.Errorf("finalize generated configuration: %w", err)
	}

	log.Printf("[INFO] Generated %s\n", path)

	return nil
}

func generatedHeader(remoteURL, localConfig string) string {
	builder := &strings.Builder{}
	builder.WriteString("# WARNING: GENERATED FILE - DO NOT EDIT\n")
	builder.WriteString("#\n")
	builder.WriteString("# Generated by golangci-wrapper.\n")
	builder.WriteString(fmt.Sprintf("# Local overrides: %s\n", localConfig))

	if remoteURL != "" {
		builder.WriteString(fmt.Sprintf("# Remote base: %s\n", remoteURL))
	} else {
		builder.WriteString("# Remote base: not configured\n")
	}

	builder.WriteString("#\n\n")

	return builder.String()
}

func buildFinalArgs(original []string, generatedConfig, originalConfig string) []string {
	finalArgs := make([]string, 0, len(original)+2)
	skipNext := false

	for i := 0; i < len(original); i++ {
		if skipNext {
			skipNext = false

			continue
		}

		arg := original[i]

		switch {
		case arg == "-c", arg == "--config":
			skipNext = true
		case strings.HasPrefix(arg, "--config="):
			// drop entirely
		default:
			finalArgs = append(finalArgs, arg)
		}
	}

	switch {
	case generatedConfig != "":
		finalArgs = append(finalArgs, "--config", generatedConfig)
	case originalConfig != "":
		finalArgs = append(finalArgs, "--config", originalConfig)
	}

	return finalArgs
}

func runGolangciLint(args []string) error {
	args = append(
		[]string{
			"tool",
			"github.com/golangci/golangci-lint/v2/cmd/golangci-lint",
		},
		args...,
	)

	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func cleanupOldGeneratedConfigs(current string) error {
	absCurrent, err := filepath.Abs(current)
	if err != nil {
		return fmt.Errorf("resolve generated config path: %w", err)
	}

	return filepath.WalkDir(".", func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			return nil
		}

		if filepath.Base(path) != generatedFileName {
			return nil
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}

		if absPath == absCurrent {
			return nil
		}

		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove old generated config %s: %w", path, err)
		}

		log.Printf("[INFO] Removed old generated config: %s\n", path)

		return nil
	})
}
