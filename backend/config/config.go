package config

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	AppVersion                 = "0.1.0"
	DefaultPort                = "3001"
	OpenRouterBaseURL          = "https://openrouter.ai/api/v1/chat/completions"
	OpenRouterModel            = "anthropic/claude-sonnet-4"
	MaxMultipartMemory         = 16 << 20
	DefaultApplicationsDirName = "applications"
	AnonymousUserID            = "anonymous-user"
)

func LoadEnvironment() {
	candidates := []string{".env", filepath.Join("..", ".env")}
	for _, path := range candidates {
		if err := LoadEnvFile(path); err == nil {
			log.Printf("Loaded environment from %s", path)
			return
		} else if !errors.Is(err, os.ErrNotExist) {
			log.Printf("Skipping %s: %v", path, err)
		}
	}
}

func LoadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if len(value) >= 2 {
			isQuoted := (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')
			if isQuoted {
				value = value[1 : len(value)-1]
			}
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	return scanner.Err()
}

func ResolveWaitlistFilePath() string {
	if DirExists("/data") {
		return "/data/waitlist.json"
	}
	return filepath.Join("state", "waitlist.json")
}

func ResolveApplicationsRootDir() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("APPLICATIONS_ROOT_PATH")); configured != "" {
		return filepath.Abs(configured)
	}
	if DirExists("/data") {
		return filepath.Abs("/data/applications")
	}

	projectRoot, err := ResolveProjectRootDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(projectRoot, DefaultApplicationsDirName), nil
}

func ResolveProjectRootDir() (string, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to determine current working directory: %w", err)
	}

	if FileExists(filepath.Join(workingDir, "backend", "go.mod")) {
		return filepath.Abs(workingDir)
	}
	if FileExists(filepath.Join(workingDir, "go.mod")) {
		parent := filepath.Dir(workingDir)
		return filepath.Abs(parent)
	}
	return filepath.Abs(workingDir)
}

func ServerAddr() string {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = DefaultPort
	}
	return "0.0.0.0:" + port
}

func GetEnv(key string) string {
	return os.Getenv(key)
}

func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
