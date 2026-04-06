package dashboard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type kubeconfigState struct {
	ActivePath    string   `json:"active_path"`
	ActiveContext string   `json:"active_context,omitempty"`
	Paths         []string `json:"paths"`
}

func defaultStatePaths() (stateFile string, uploadDir string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("resolving user home: %w", err)
	}
	baseDir := filepath.Join(home, ".kubepilot")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", "", fmt.Errorf("creating state dir: %w", err)
	}
	uploadDir = filepath.Join(baseDir, "kubeconfigs")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return "", "", fmt.Errorf("creating upload dir: %w", err)
	}
	return filepath.Join(baseDir, "kubeconfigs.json"), uploadDir, nil
}

func loadKubeconfigState(path string) (*kubeconfigState, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &kubeconfigState{Paths: []string{}}, nil
		}
		return nil, fmt.Errorf("reading kubeconfig state: %w", err)
	}

	var st kubeconfigState
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("parsing kubeconfig state: %w", err)
	}
	if st.Paths == nil {
		st.Paths = []string{}
	}
	return &st, nil
}

func saveKubeconfigState(path string, st *kubeconfigState) error {
	// Keep persisted path list stable and deduplicated for predictable UX.
	seen := map[string]struct{}{}
	paths := make([]string, 0, len(st.Paths))
	for _, p := range st.Paths {
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}
	sort.Strings(paths)
	st.Paths = paths

	out, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding kubeconfig state: %w", err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("writing kubeconfig state: %w", err)
	}
	return nil
}

func expandKubeconfigPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}
