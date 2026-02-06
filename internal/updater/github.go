package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	gitHubAPITimeout = 10 * time.Second
)

// GitHubRelease represents a GitHub release.
type GitHubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	PublishedAt string `json:"published_at"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
}

// GetLatestRelease fetches the latest release from a GitHub repository.
func GetLatestRelease(repo string) (*GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)

	client := &http.Client{
		Timeout: gitHubAPITimeout,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "dnstm-updater")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("GitHub API rate limit exceeded (try again later)")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d for %s", resp.StatusCode, repo)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	return &release, nil
}

// GetDnstmLatestVersion fetches the latest dnstm version from GitHub.
func GetDnstmLatestVersion() (string, error) {
	release, err := GetLatestRelease("net2share/dnstm")
	if err != nil {
		return "", err
	}
	return release.TagName, nil
}
