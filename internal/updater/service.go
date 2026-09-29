package updater

import (
	"ADBKit/internal/core"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const latestReleaseURL = "https://api.github.com/repos/mrAibo/tvadb-hub/releases/latest"

type ReleaseInfo struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL      string `json:"releaseUrl"`
	ReleaseName     string `json:"releaseName"`
	PublishedAt     string `json:"publishedAt"`
}

type Service struct {
	client   *http.Client
	endpoint string
}

func NewService() *Service {
	return &Service{
		client: &http.Client{Timeout: 10 * time.Second},
		endpoint: latestReleaseURL,
	}
}

func (s *Service) Check(ctx context.Context) (ReleaseInfo, error) {
	current := core.Version

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.endpoint, nil)
	if err != nil {
		return ReleaseInfo{}, core.NewOperationError("check_for_updates", "Failed to prepare update check", err.Error(), true)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "DroidSphere/"+current)

	resp, err := s.client.Do(req)
	if err != nil {
		return ReleaseInfo{}, core.NewOperationError("check_for_updates", "Could not reach GitHub Releases", err.Error(), true)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ReleaseInfo{}, core.NewOperationError(
			"check_for_updates",
			"Could not read the latest DroidSphere release",
			fmt.Sprintf("GitHub Releases returned HTTP %d", resp.StatusCode),
			true,
		)
	}

	var payload struct {
		TagName     string `json:"tag_name"`
		HTMLURL     string `json:"html_url"`
		Name        string `json:"name"`
		PublishedAt string `json:"published_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ReleaseInfo{}, core.NewOperationError("check_for_updates", "Invalid GitHub release response", err.Error(), true)
	}

	latest := normalizeVersion(payload.TagName)
	if latest == "" {
		return ReleaseInfo{}, core.NewOperationError(
			"check_for_updates",
			"Latest release has an invalid version",
			payload.TagName,
			true,
		)
	}

	cmp, err := compareVersions(latest, normalizeVersion(current))
	if err != nil {
		return ReleaseInfo{}, core.NewOperationError("check_for_updates", "Could not compare release versions", err.Error(), false)
	}

	return ReleaseInfo{
		CurrentVersion: current,
		LatestVersion: latest,
		UpdateAvailable: cmp > 0,
		ReleaseURL: payload.HTMLURL,
		ReleaseName: payload.Name,
		PublishedAt: payload.PublishedAt,
	}, nil
}

func normalizeVersion(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), "v"))
	if value == "" {
		return ""
	}
	if idx := strings.IndexAny(value, "-+"); idx >= 0 {
		value = value[:idx]
	}
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return ""
	}
	for _, part := range parts {
		if part == "" {
			return ""
		}
		if _, err := strconv.Atoi(part); err != nil {
			return ""
		}
	}
	return strings.Join(parts, ".")
}

func compareVersions(a string, b string) (int, error) {
	parse := func(value string) ([3]int, error) {
		var out [3]int
		normalized := normalizeVersion(value)
		if normalized == "" {
			return out, fmt.Errorf("invalid semantic version %q", value)
		}
		for i, part := range strings.Split(normalized, ".") {
			n, err := strconv.Atoi(part)
			if err != nil {
				return out, err
			}
			out[i] = n
		}
		return out, nil
	}

	av, err := parse(a)
	if err != nil {
		return 0, err
	}
	bv, err := parse(b)
	if err != nil {
		return 0, err
	}
	for i := range av {
		switch {
		case av[i] > bv[i]:
			return 1, nil
		case av[i] < bv[i]:
			return -1, nil
		}
	}
	return 0, nil
}
