package monitor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// StatsFilePath returns the path to the stats JSON file for a tunnel tag.
func StatsFilePath(tag string) string {
	return filepath.Join(RunDir, tag+"-stats.json")
}

// ReadStats reads the accumulated stats from the sniffer's JSON file.
// Returns nil if the file doesn't exist (sniffer not running).
func ReadStats(tag string) (*CaptureResult, error) {
	path := StatsFilePath(tag)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read stats: %w", err)
	}

	var result CaptureResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse stats: %w", err)
	}

	return &result, nil
}
