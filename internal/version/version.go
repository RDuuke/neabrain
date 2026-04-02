package version

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Current is the version embedded at build time.
const Current = "0.2.0"

// releaseURL is the GitHub API endpoint for the latest release.
const releaseURL = "https://api.github.com/repos/RDuuke/neabrain/releases/latest"

// CheckResult holds the result of a version check.
type CheckResult struct {
	Current   string
	Latest    string
	UpToDate  bool
	UpdateCmd string
}

// Check queries GitHub for the latest release and compares it with Current.
func Check(ctx context.Context) (CheckResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return CheckResult{}, fmt.Errorf("version check: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "neabrain/"+Current)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return CheckResult{}, fmt.Errorf("version check: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return CheckResult{}, fmt.Errorf("version check: GitHub API returned %d", resp.StatusCode)
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return CheckResult{}, fmt.Errorf("version check: decode response: %w", err)
	}

	latest := strings.TrimPrefix(payload.TagName, "v")
	upToDate := compare(Current, latest) >= 0

	result := CheckResult{
		Current:  Current,
		Latest:   latest,
		UpToDate: upToDate,
	}
	if !upToDate {
		result.UpdateCmd = updateInstruction(latest)
	}
	return result, nil
}

// updateInstruction returns the platform-appropriate install command.
func updateInstruction(latest string) string {
	switch runtime.GOOS {
	case "windows":
		return fmt.Sprintf("go install github.com/RDuuke/neabrain/cmd/neabrain@v%s", latest)
	case "darwin":
		return fmt.Sprintf("go install github.com/RDuuke/neabrain/cmd/neabrain@v%s", latest)
	default:
		return fmt.Sprintf("go install github.com/RDuuke/neabrain/cmd/neabrain@v%s", latest)
	}
}

// compare returns -1, 0, or 1 when a < b, a == b, a > b respectively.
// Expects semver strings like "1.2.3" (no leading 'v').
func compare(a, b string) int {
	ap := parseSemver(a)
	bp := parseSemver(b)
	for i := 0; i < 3; i++ {
		if ap[i] < bp[i] {
			return -1
		}
		if ap[i] > bp[i] {
			return 1
		}
	}
	return 0
}

func parseSemver(v string) [3]int {
	parts := strings.SplitN(v, ".", 3)
	var out [3]int
	for i, p := range parts {
		if i >= 3 {
			break
		}
		n, _ := strconv.Atoi(p)
		out[i] = n
	}
	return out
}
