package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
)

func resolveRegions(tmpConfigPath, profile, discoveryRegion, regionsArg string, includeAllRegions bool) ([]string, error) {
	if strings.TrimSpace(regionsArg) != "" {
		parts := strings.Split(regionsArg, ",")
		var regions []string
		seen := map[string]struct{}{}
		for _, p := range parts {
			region := strings.TrimSpace(p)
			if region == "" {
				continue
			}
			if _, ok := seen[region]; ok {
				continue
			}
			seen[region] = struct{}{}
			regions = append(regions, region)
		}
		return regions, nil
	}
	args := []string{
		"ec2", "describe-regions",
		"--region", discoveryRegion,
	}
	if includeAllRegions {
		args = append(args, "--all-regions")
	}
	out, err := runAWSJSON(tmpConfigPath, profile, args)
	if err != nil {
		return nil, err
	}
	var resp ec2DescribeRegionsResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("decode describe-regions: %w", err)
	}
	var regions []string
	for _, r := range resp.Regions {
		if strings.TrimSpace(r.RegionName) != "" {
			regions = append(regions, r.RegionName)
		}
	}
	sort.Strings(regions)
	return regions, nil
}

func buildTemporaryAWSConfig(base profileConfig, targets []roleTarget) (string, map[string]string, error) {
	content, err := os.ReadFile(awsConfigPath())
	if err != nil {
		return "", nil, fmt.Errorf("read ~/.aws/config: %w", err)
	}

	var buf bytes.Buffer
	buf.Write(content)
	if !bytes.HasSuffix(content, []byte("\n")) {
		buf.WriteString("\n")
	}

	profileNames := make(map[string]string)
	for i, t := range targets {
		profileName := fmt.Sprintf("swamp-%d", i+1)
		profileNames[targetKey(t)] = profileName

		buf.WriteString(fmt.Sprintf("\n[profile %s]\n", profileName))
		if base.SSOSession != "" {
			buf.WriteString(fmt.Sprintf("sso_session = %s\n", base.SSOSession))
		} else {
			buf.WriteString(fmt.Sprintf("sso_start_url = %s\n", base.SSOStartURL))
			buf.WriteString(fmt.Sprintf("sso_region = %s\n", base.SSORegion))
		}
		buf.WriteString(fmt.Sprintf("sso_account_id = %s\n", t.AccountID))
		buf.WriteString(fmt.Sprintf("sso_role_name = %s\n", t.RoleName))
		if base.Region != "" {
			buf.WriteString(fmt.Sprintf("region = %s\n", base.Region))
		} else {
			buf.WriteString("region = us-east-1\n")
		}
		if base.Output != "" {
			buf.WriteString(fmt.Sprintf("output = %s\n", base.Output))
		} else {
			buf.WriteString("output = json\n")
		}
	}

	f, err := os.CreateTemp("", "aws-config-swamp-*.ini")
	if err != nil {
		return "", nil, fmt.Errorf("create temp config: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(buf.Bytes()); err != nil {
		return "", nil, fmt.Errorf("write temp config: %w", err)
	}
	return f.Name(), profileNames, nil
}

func scanAllInstances(tmpConfigPath string, targets []roleTarget, profileNames map[string]string, regions []string, workers int, runningOnly bool) []instanceCandidate {
	type job struct {
		target  roleTarget
		profile string
		region  string
	}

	jobs := make(chan job, workers*2)
	results := make(chan scanResult, workers*2)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				cands, _ := queryInstances(tmpConfigPath, j.target, j.profile, j.region, runningOnly)
				if len(cands) > 0 {
					results <- scanResult{Candidates: cands}
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	go func() {
		for _, t := range targets {
			profile := profileNames[targetKey(t)]
			for _, region := range regions {
				jobs <- job{
					target:  t,
					profile: profile,
					region:  region,
				}
			}
		}
		close(jobs)
	}()

	var all []instanceCandidate
	for r := range results {
		all = append(all, r.Candidates...)
	}
	return all
}

func queryInstances(tmpConfigPath string, target roleTarget, profileName, region string, runningOnly bool) ([]instanceCandidate, error) {
	args := []string{"ec2", "describe-instances", "--region", region}
	if runningOnly {
		args = append(args, "--filters", "Name=instance-state-name,Values=running")
	}
	out, err := runAWSJSON(tmpConfigPath, profileName, args)
	if err != nil {
		// Silently ignore combinations that are not viable in this account/role/region.
		return nil, err
	}
	var resp ec2DescribeInstancesResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("decode describe-instances: %w", err)
	}

	var candidates []instanceCandidate
	for _, res := range resp.Reservations {
		for _, inst := range res.Instances {
			if strings.TrimSpace(inst.InstanceID) == "" {
				continue
			}
			name := findTag(inst.Tags, "Name")
			if name == "" {
				name = "-"
			}
			ip := inst.PrivateIP
			if ip == "" {
				ip = "-"
			}
			state := inst.State.Name
			if state == "" {
				state = "-"
			}
			platform := inst.PlatformDetails
			if platform == "" {
				platform = "-"
			}
			line := fmt.Sprintf("%s | %s | %s | %s | %s | %s | %s",
				target.AccountName, target.AccountID, target.RoleName, region, inst.InstanceID, name, ip)
			if !runningOnly {
				line = fmt.Sprintf("%s | state=%s | platform=%s", line, state, platform)
			}
			candidates = append(candidates, instanceCandidate{
				DisplayLine: line,
				ProfileName: profileName,
				Region:      region,
				InstanceID:  inst.InstanceID,
			})
		}
	}
	return candidates, nil
}

func startSSMSession(tmpConfigPath, profile, region, instanceID string) error {
	cmd := exec.Command("aws",
		"--profile", profile,
		"--region", region,
		"ssm", "start-session",
		"--target", instanceID,
	)
	cmd.Env = append(os.Environ(),
		"AWS_SDK_LOAD_CONFIG=1",
		"AWS_CONFIG_FILE="+tmpConfigPath,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	for {
		select {
		case err := <-waitCh:
			return err
		case sig := <-sigCh:
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}
}

func runAWSJSON(tmpConfigPath, profile string, args []string) ([]byte, error) {
	fullArgs := []string{}
	if strings.TrimSpace(profile) != "" {
		fullArgs = append(fullArgs, "--profile", profile)
	}
	fullArgs = append(fullArgs, args...)
	fullArgs = append(fullArgs, "--output", "json")

	cmd := exec.Command("aws", fullArgs...)
	if strings.TrimSpace(tmpConfigPath) != "" {
		cmd.Env = append(os.Environ(),
			"AWS_SDK_LOAD_CONFIG=1",
			"AWS_CONFIG_FILE="+tmpConfigPath,
		)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s (aws %s)", msg, strings.Join(redactSensitiveArgs(fullArgs), " "))
	}
	return out, nil
}

func redactSensitiveArgs(args []string) []string {
	out := append([]string(nil), args...)
	for i := 0; i < len(out); i++ {
		if out[i] == "--access-token" && i+1 < len(out) {
			out[i+1] = "<redacted>"
			i++
		}
	}
	return out
}
