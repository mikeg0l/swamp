package app

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func readProfileConfig(profile string) (profileConfig, error) {
	path := awsConfigPath()
	f, err := os.Open(path)
	if err != nil {
		return profileConfig{}, err
	}
	defer f.Close()

	targetSection := fmt.Sprintf("profile %s", profile)
	currentSection := ""
	cfg := profileConfig{Name: profile}
	sections := map[string]map[string]string{}
	profileExists := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			continue
		}
		if currentSection == targetSection {
			profileExists = true
		}
		if currentSection == "" {
			continue
		}
		key, val, ok := splitKeyValue(line)
		if !ok {
			continue
		}
		key = strings.ToLower(key)
		if _, ok := sections[currentSection]; !ok {
			sections[currentSection] = map[string]string{}
		}
		sections[currentSection][key] = val
	}
	if err := scanner.Err(); err != nil {
		return profileConfig{}, err
	}
	cfg.SourceExists = profileExists
	if !profileExists {
		return cfg, nil
	}

	if profileValues, ok := sections[targetSection]; ok {
		cfg.Region = profileValues["region"]
		cfg.Output = profileValues["output"]
		cfg.SSOSession = profileValues["sso_session"]
		cfg.SSOStartURL = profileValues["sso_start_url"]
		cfg.SSORegion = profileValues["sso_region"]
	}

	if cfg.SSOSession != "" {
		sessionSection := "sso-session " + cfg.SSOSession
		if sessionValues, ok := sections[sessionSection]; ok {
			if cfg.SSOStartURL == "" {
				cfg.SSOStartURL = sessionValues["sso_start_url"]
			}
			if cfg.SSORegion == "" {
				cfg.SSORegion = sessionValues["sso_region"]
			}
		}
	}

	if cfg.SSORegion == "" {
		cfg.SSORegion = cfg.Region
	}

	if cfg.SSOSession == "" && (cfg.SSOStartURL == "" || cfg.SSORegion == "") {
		return profileConfig{}, fmt.Errorf("profile %q is not configured as an SSO profile", profile)
	}
	return cfg, nil
}

func splitKeyValue(line string) (key, val string, ok bool) {
	idx := strings.Index(line, "=")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	val = strings.TrimSpace(line[idx+1:])
	if key == "" || val == "" {
		return "", "", false
	}
	return key, val, true
}

func awsConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".aws/config"
	}
	return filepath.Join(home, ".aws", "config")
}

func awsSSOCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".aws/sso/cache"
	}
	return filepath.Join(home, ".aws", "sso", "cache")
}
