package app

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func selectAccountWithFZF(accounts []ssoAccountsResponse) (*ssoAccountsResponse, error) {
	lookup := make(map[string]ssoAccountsResponse, len(accounts))
	lines := make([]string, 0, len(accounts))
	for _, a := range accounts {
		if len(a.AccountList) == 0 {
			continue
		}
		acct := a.AccountList[0]
		line := fmt.Sprintf("%s | %s | %s", acct.AccountName, acct.AccountID, acct.EmailAddress)
		lines = append(lines, line)
		lookup[line] = a
	}
	if len(lines) == 0 {
		return nil, nil
	}
	selected, ok, err := pickLineWithFZF(lines, "Select account > ")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	chosen, found := lookup[selected]
	if !found {
		return nil, fmt.Errorf("selected account not found")
	}
	return &chosen, nil
}

func selectRoleTargetWithFZF(targets []roleTarget) (*roleTarget, error) {
	lookup := make(map[string]roleTarget, len(targets))
	lines := make([]string, 0, len(targets))
	for _, t := range targets {
		line := fmt.Sprintf("%s | %s | %s", t.AccountName, t.AccountID, t.RoleName)
		lines = append(lines, line)
		lookup[line] = t
	}
	if len(lines) == 0 {
		return nil, nil
	}
	selected, ok, err := pickLineWithFZF(lines, "Select role > ")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	chosen, found := lookup[selected]
	if !found {
		return nil, fmt.Errorf("selected role not found")
	}
	return &chosen, nil
}

func selectRegionWithFZF(regions []string) (string, error) {
	lookup := make(map[string]string, len(regions))
	lines := make([]string, 0, len(regions))
	for _, region := range regions {
		line := fmt.Sprintf("%s | %s", region, regionDisplayName(region))
		lines = append(lines, line)
		lookup[line] = region
	}

	selected, ok, err := pickLineWithFZF(lines, "Select region > ")
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	region, found := lookup[selected]
	if !found {
		return "", fmt.Errorf("selected region not found")
	}
	return region, nil
}

func pickLineWithFZF(lines []string, prompt string) (string, bool, error) {
	var in bytes.Buffer
	for _, line := range lines {
		in.WriteString(line)
		in.WriteString("\n")
	}

	cmd := exec.Command("fzf", "--height", "80%", "--layout", "reverse", "--prompt", prompt)
	cmd.Stdin = &in
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 130 {
			return "", false, nil
		}
		return "", false, err
	}
	selected := strings.TrimSpace(out.String())
	if selected == "" {
		return "", false, nil
	}
	return selected, true, nil
}

func pickWithFZF(candidates []instanceCandidate) (*instanceCandidate, error) {
	var in bytes.Buffer
	lookup := make(map[string]instanceCandidate, len(candidates))
	for _, c := range candidates {
		in.WriteString(c.DisplayLine)
		in.WriteString("\n")
		lookup[c.DisplayLine] = c
	}

	cmd := exec.Command("fzf", "--ansi", "--height", "80%", "--layout", "reverse", "--prompt", "Select EC2 instance > ")
	cmd.Stdin = &in
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 130 {
			return nil, nil
		}
		return nil, err
	}

	selectedLine := strings.TrimSpace(out.String())
	if selectedLine == "" {
		return nil, nil
	}
	selected, ok := lookup[selectedLine]
	if !ok {
		return nil, fmt.Errorf("selected value not found in lookup")
	}
	return &selected, nil
}
