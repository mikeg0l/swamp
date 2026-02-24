package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const recentTargetsFileName = "recent_targets.json"

type recentTargetsFile struct {
	Version  int                          `json:"version"`
	Profiles map[string]recentProfileData `json:"profiles"`
}

type recentProfileData struct {
	LastScope    recentScope    `json:"last_scope"`
	LastInstance recentInstance `json:"last_instance"`
	UpdatedAt    string         `json:"updated_at"`
}

type recentScope struct {
	AccountID   string `json:"account_id"`
	AccountName string `json:"account_name"`
	RoleName    string `json:"role_name"`
	Region      string `json:"region"`
}

type recentInstance struct {
	InstanceID  string `json:"instance_id"`
	Region      string `json:"region"`
	ProfileName string `json:"profile_name"`
	DisplayLine string `json:"display_line"`
}

func loadRecentTargets(cacheDir string) (recentTargetsFile, error) {
	path := filepath.Join(cacheDir, recentTargetsFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return recentTargetsFile{Version: 1, Profiles: map[string]recentProfileData{}}, nil
		}
		return recentTargetsFile{}, err
	}
	var out recentTargetsFile
	if err := json.Unmarshal(data, &out); err != nil {
		return recentTargetsFile{Version: 1, Profiles: map[string]recentProfileData{}}, nil
	}
	if out.Version == 0 {
		out.Version = 1
	}
	if out.Profiles == nil {
		out.Profiles = map[string]recentProfileData{}
	}
	return out, nil
}

func saveRecentTargets(cacheDir, profile string, scope recentScope, inst recentInstance) error {
	if strings.TrimSpace(cacheDir) == "" || strings.TrimSpace(profile) == "" {
		return nil
	}
	all, err := loadRecentTargets(cacheDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	all.Profiles[profile] = recentProfileData{
		LastScope:    scope,
		LastInstance: inst,
		UpdatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	content, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(cacheDir, "recent-targets-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	finalPath := filepath.Join(cacheDir, recentTargetsFileName)
	if err := os.Rename(tmpName, finalPath); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func (f recentTargetsFile) getLastScope(profile string) (recentScope, bool) {
	p, ok := f.Profiles[profile]
	if !ok {
		return recentScope{}, false
	}
	if strings.TrimSpace(p.LastScope.AccountID) == "" || strings.TrimSpace(p.LastScope.RoleName) == "" {
		return recentScope{}, false
	}
	return p.LastScope, true
}

func (f recentTargetsFile) getLastInstance(profile string) (recentScope, recentInstance, bool) {
	p, ok := f.Profiles[profile]
	if !ok {
		return recentScope{}, recentInstance{}, false
	}
	if strings.TrimSpace(p.LastScope.AccountID) == "" ||
		strings.TrimSpace(p.LastScope.RoleName) == "" ||
		strings.TrimSpace(p.LastInstance.InstanceID) == "" ||
		strings.TrimSpace(p.LastInstance.Region) == "" {
		return recentScope{}, recentInstance{}, false
	}
	return p.LastScope, p.LastInstance, true
}

func tryLastConnection(opts Options, cfg profileConfig, recent recentTargetsFile, ssoRegion string) (bool, error) {
	scope, inst, ok := recent.getLastInstance(opts.Profile)
	if !ok {
		fmt.Printf("No recent target found for profile %q; continuing interactively.\n", opts.Profile)
		return false, nil
	}
	target := roleTarget{
		AccountID:   scope.AccountID,
		AccountName: scope.AccountName,
		RoleName:    scope.RoleName,
	}
	tmpConfigPath, profileNames, err := buildTempAWSConfigFn(cfg, []roleTarget{target})
	if err != nil {
		return false, fmt.Errorf("failed to build temporary AWS config: %w", err)
	}
	defer func() {
		_ = removeFileFn(tmpConfigPath)
	}()

	regions, err := discoverRegionsFn(opts, cfg, []roleTarget{target}, tmpConfigPath, profileNames, ssoRegion)
	if err != nil {
		fmt.Printf("Saved target no longer available (%v); continuing interactively.\n", err)
		return false, nil
	}
	if !containsString(regions, inst.Region) {
		fmt.Printf("Saved target region %q is no longer available; continuing interactively.\n", inst.Region)
		return false, nil
	}

	candidates := scanAllInstancesFn(opts, tmpConfigPath, []roleTarget{target}, profileNames, []string{inst.Region}, opts.Workers, !opts.IncludeStopped)
	var selected *instanceCandidate
	for i := range candidates {
		if candidates[i].InstanceID == inst.InstanceID {
			selected = &candidates[i]
			break
		}
	}
	if selected == nil {
		fmt.Printf("Saved instance %q was not found; continuing interactively.\n", inst.InstanceID)
		return false, nil
	}

	fmt.Printf("Starting SSM session to %s in %s (profile %s)\n", selected.InstanceID, selected.Region, selected.ProfileName)
	if err := startSSMSessionFn(tmpConfigPath, selected.ProfileName, selected.Region, selected.InstanceID); err != nil {
		fmt.Printf("Saved target connection failed (%v); continuing interactively.\n", err)
		return false, nil
	}
	_ = saveRecentTargets(opts.CacheDir, opts.Profile, scope, recentInstance{
		InstanceID:  selected.InstanceID,
		Region:      selected.Region,
		ProfileName: selected.ProfileName,
		DisplayLine: selected.DisplayLine,
	})
	return true, nil
}

func containsString(items []string, needle string) bool {
	for _, v := range items {
		if v == needle {
			return true
		}
	}
	return false
}
