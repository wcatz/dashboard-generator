package generator

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// WriteDashboard writes a dashboard to JSON file, returning the size.
func WriteDashboard(dashboard map[string]interface{}, fpath string, dryRun bool) (int, error) {
	data, err := json.MarshalIndent(dashboard, "", "  ")
	if err != nil {
		return 0, fmt.Errorf("marshaling dashboard: %w", err)
	}
	data = append(data, '\n')
	size := len(data)
	panelCount := countPanels(dashboard)
	filename := filepath.Base(fpath)

	if size > 750_000 {
		fmt.Fprintf(os.Stderr, "  WARNING: %s is %s bytes (>750KB ConfigMap limit)\n", filename, formatSize(size))
	}

	if !dryRun {
		if err := os.WriteFile(fpath, data, 0644); err != nil {
			return 0, fmt.Errorf("writing %s: %w", fpath, err)
		}
	}

	fmt.Printf("  %s: %d panels, %s bytes\n", filename, panelCount, formatSize(size))
	return size, nil
}

func countPanels(dashboard map[string]interface{}) int {
	panels, ok := dashboard["panels"].([]interface{})
	if !ok {
		return 0
	}
	return len(panels)
}

func formatSize(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	// insert commas
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

// PushToGrafana pushes a dashboard to the Grafana API.
func PushToGrafana(dashboard map[string]interface{}, grafanaURL, authUser, authPass, token string) error {
	payload := map[string]interface{}{
		"dashboard": dashboard,
		"overwrite": true,
		"message":   "updated by grafana-dashboard-generator",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	url := fmt.Sprintf("%s/api/dashboards/db", trimSlash(grafanaURL))
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	} else if authUser != "" && authPass != "" {
		creds := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", authUser, authPass)))
		req.Header.Set("Authorization", fmt.Sprintf("Basic %s", creds))
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("pushing dashboard: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("grafana returned %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err == nil {
		status := "unknown"
		if s, ok := result["status"].(string); ok {
			status = s
		}
		uid := "?"
		if u, ok := result["uid"].(string); ok {
			uid = u
		} else if u, ok := dashboard["uid"].(string); ok {
			uid = u
		}
		fmt.Printf("  pushed %s: %s\n", uid, status)
	}

	return nil
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
