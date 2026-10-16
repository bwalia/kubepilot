package dashboard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/kubepilot/kubepilot/pkg/ai"
	"github.com/kubepilot/kubepilot/pkg/k8s"
	"github.com/kubepilot/kubepilot/pkg/runbooks"
	"github.com/kubepilot/kubepilot/pkg/security"
)

// ClusterHeader selects the cluster a request runs against. Each browser tab
// or mobile session sends its own choice, so two people looking at different
// clusters never move each other's view. Without it a request uses the
// server's default cluster, which is also what background work (Autopilot,
// RCA, OTLP) watches.
const ClusterHeader = "X-KubePilot-Cluster"

// clusterQueryParam is the fallback for links that cannot set headers.
const clusterQueryParam = "cluster"

// inClusterID names the cluster KubePilot runs in when it has no kubeconfig.
const inClusterID = "in-cluster"

// ClusterInfo is one selectable cluster: a context inside a known kubeconfig.
type ClusterInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Context    string `json:"context"`
	Cluster    string `json:"cluster"`
	Server     string `json:"server,omitempty"`
	Kubeconfig string `json:"kubeconfig"`
	Default    bool   `json:"default"`
	// Reachable, Version and Error are only filled when the caller asks
	// for a probe; nil means "not checked".
	Reachable *bool  `json:"reachable,omitempty"`
	Version   string `json:"version,omitempty"`
	Error     string `json:"error,omitempty"`
}

// unavailableKubeconfig is a saved path that can no longer be read, listed so
// the UI can offer to forget it.
type unavailableKubeconfig struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// clusterHandle is everything a request needs to act on one cluster.
type clusterHandle struct {
	id         string
	kubeconfig string
	context    string
	client     *k8s.Client
	guard      *security.Guard
}

type clusterCtxKey struct{}

// clusterID derives a stable, opaque ID from a kubeconfig path and context,
// so the ID survives restarts and never exposes the server's file layout.
func clusterID(kubeconfigPath, contextName string) string {
	if kubeconfigPath == "" {
		return inClusterID
	}
	sum := sha256.Sum256([]byte(kubeconfigPath + "\x00" + contextName))
	return hex.EncodeToString(sum[:6])
}

// clusterRegistry caches one client per selected cluster. Building a client
// is cheap but not free, and reusing it keeps client-go's connection pool.
type clusterRegistry struct {
	mu      sync.Mutex
	handles map[string]*clusterHandle
}

func (c *clusterRegistry) get(id string) *clusterHandle {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.handles[id]
}

func (c *clusterRegistry) put(h *clusterHandle) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handles == nil {
		c.handles = map[string]*clusterHandle{}
	}
	c.handles[h.id] = h
}

// forgetPath drops cached clients built from a kubeconfig, so a file that was
// re-uploaded or removed is read again on next use.
func (c *clusterRegistry) forgetPath(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for id, h := range c.handles {
		if h.kubeconfig == path {
			delete(c.handles, id)
		}
	}
}

// listClusters expands every known kubeconfig into its contexts.
func (s *Server) listClusters() ([]ClusterInfo, []unavailableKubeconfig, string) {
	s.mu.RLock()
	activePath := s.activeKubeconfigPath
	activeCtx := s.activeContext
	paths := make([]string, 0, len(s.knownKubeconfigPaths))
	for p := range s.knownKubeconfigPaths {
		paths = append(paths, p)
	}
	s.mu.RUnlock()
	sort.Strings(paths)

	if activePath == "" && len(paths) == 0 {
		return []ClusterInfo{{
			ID: inClusterID, Name: inClusterID, Context: inClusterID, Cluster: inClusterID, Default: true,
		}}, []unavailableKubeconfig{}, inClusterID
	}

	defaultID := ""
	clusters := []ClusterInfo{}
	unavailable := []unavailableKubeconfig{}
	for _, p := range paths {
		raw, err := clientcmd.LoadFromFile(p)
		if err != nil {
			unavailable = append(unavailable, unavailableKubeconfig{Path: p, Error: err.Error()})
			continue
		}
		names := make([]string, 0, len(raw.Contexts))
		for name := range raw.Contexts {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			kctx := raw.Contexts[name]
			info := ClusterInfo{
				ID:         clusterID(p, name),
				Context:    name,
				Cluster:    kctx.Cluster,
				Kubeconfig: p,
			}
			if cl, ok := raw.Clusters[kctx.Cluster]; ok {
				info.Server = cl.Server
			}
			if p == activePath {
				want := activeCtx
				if want == "" {
					want = raw.CurrentContext
				}
				if name == want {
					info.Default = true
					defaultID = info.ID
				}
			}
			clusters = append(clusters, info)
		}
	}
	nameClusters(clusters)
	return clusters, unavailable, defaultID
}

// nameClusters gives each cluster a short label. k3s and kind write every
// kubeconfig with a context called "default", so a bare context name is
// often ambiguous; the file name tells those apart.
func nameClusters(clusters []ClusterInfo) {
	counts := map[string]int{}
	for _, c := range clusters {
		counts[c.Context]++
	}
	for i := range clusters {
		c := &clusters[i]
		stem := strings.TrimSuffix(filepath.Base(c.Kubeconfig), filepath.Ext(c.Kubeconfig))
		switch {
		case c.Context == "default":
			c.Name = stem
		case counts[c.Context] > 1:
			c.Name = stem + "/" + c.Context
		default:
			c.Name = c.Context
		}
	}
	// A file holding two "default"-like contexts can still collide.
	seen := map[string]int{}
	for i := range clusters {
		seen[clusters[i].Name]++
	}
	for i := range clusters {
		if seen[clusters[i].Name] > 1 {
			clusters[i].Name += " (" + clusters[i].ID[:4] + ")"
		}
	}
}

// resolveCluster returns the handle for a cluster ID, building and caching a
// client on first use. An empty ID or the default's ID returns nil, meaning
// "use the server default".
func (s *Server) resolveCluster(id string) (*clusterHandle, error) {
	if id == "" {
		return nil, nil
	}
	if h := s.clusters.get(id); h != nil {
		return h, nil
	}
	clusters, _, defaultID := s.listClusters()
	if id == defaultID {
		return nil, nil
	}
	for _, c := range clusters {
		if c.ID != id {
			continue
		}
		client, err := k8s.NewClientWithContext(c.Kubeconfig, c.Context)
		if err != nil {
			return nil, fmt.Errorf("connecting to cluster %q: %w", c.Name, err)
		}
		h := &clusterHandle{
			id: id, kubeconfig: c.Kubeconfig, context: c.Context,
			client: client, guard: security.NewGuard(client.Core, s.log),
		}
		s.clusters.put(h)
		return h, nil
	}
	return nil, errUnknownCluster
}

var errUnknownCluster = errors.New("unknown cluster; it may have been removed — pick another from the cluster list")

// withClusterSelection resolves the requested cluster once per request and
// stores it in the request context for the handlers.
func (s *Server) withClusterSelection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get(ClusterHeader))
		if id == "" {
			id = strings.TrimSpace(r.URL.Query().Get(clusterQueryParam))
		}
		h, err := s.resolveCluster(id)
		if err != nil {
			status := http.StatusBadGateway
			if errors.Is(err, errUnknownCluster) {
				status = http.StatusConflict
			}
			httpError(w, err, status)
			return
		}
		if h != nil {
			r = r.WithContext(context.WithValue(r.Context(), clusterCtxKey{}, h))
		}
		next.ServeHTTP(w, r)
	})
}

func selectedCluster(r *http.Request) *clusterHandle {
	if r == nil {
		return nil
	}
	h, _ := r.Context().Value(clusterCtxKey{}).(*clusterHandle)
	return h
}

// k8sFor is the Kubernetes client for the cluster this request selected.
func (s *Server) k8sFor(r *http.Request) *k8s.Client {
	if h := selectedCluster(r); h != nil {
		return h.client
	}
	return s.currentK8sClient()
}

// guardFor validates CR codes against the selected cluster's secrets.
func (s *Server) guardFor(r *http.Request) *security.Guard {
	if h := selectedCluster(r); h != nil {
		return h.guard
	}
	return s.currentGuard()
}

// aiFor is the AI engine bound to the selected cluster.
func (s *Server) aiFor(r *http.Request) *ai.Engine {
	if h := selectedCluster(r); h != nil && s.cfg.AIEngine != nil {
		return s.cfg.AIEngine.WithK8sClient(h.client)
	}
	return s.cfg.AIEngine
}

// runbooksFor is the runbook engine bound to the selected cluster.
func (s *Server) runbooksFor(r *http.Request) *runbooks.Engine {
	if h := selectedCluster(r); h != nil {
		return s.cfg.RunbookEngine.WithK8sClient(h.client, s.aiFor(r))
	}
	return s.cfg.RunbookEngine
}

// kubeconfigFor is the kubeconfig path and context kubectl should use.
func (s *Server) kubeconfigFor(r *http.Request) (path, contextName string) {
	if h := selectedCluster(r); h != nil {
		return h.kubeconfig, h.context
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeKubeconfigPath, s.activeContext
}

// handleListClusters returns every selectable cluster. With ?probe=true it
// also checks each API server, concurrently and with a short timeout, so one
// dead cluster cannot stall the list.
func (s *Server) handleListClusters(w http.ResponseWriter, r *http.Request) {
	clusters, unavailable, defaultID := s.listClusters()
	if probe := r.URL.Query().Get("probe"); probe == "true" || probe == "1" {
		s.probeClusters(r.Context(), clusters, defaultID)
	}
	writeJSON(w, map[string]any{
		"default_id":  defaultID,
		"clusters":    clusters,
		"unavailable": unavailable,
		"mutations":   s.cfg.EnableKubeconfigMutationEndpoints,
		"header":      ClusterHeader,
	})
}

func (s *Server) probeClusters(ctx context.Context, clusters []ClusterInfo, defaultID string) {
	var wg sync.WaitGroup
	for i := range clusters {
		wg.Add(1)
		go func(c *ClusterInfo) {
			defer wg.Done()
			var client *k8s.Client
			if c.ID == defaultID || c.ID == inClusterID {
				client = s.currentK8sClient()
			} else if h, err := s.resolveCluster(c.ID); err != nil {
				c.Error = err.Error()
			} else if h != nil {
				client = h.client
			}
			ok := false
			if client != nil {
				pctx, cancel := context.WithTimeout(ctx, 4*time.Second)
				defer cancel()
				body, err := client.Core.Discovery().RESTClient().Get().AbsPath("/version").Do(pctx).Raw()
				if err != nil {
					c.Error = err.Error()
				} else {
					ok = true
					c.Version = parseGitVersion(body)
				}
			}
			c.Reachable = &ok
		}(&clusters[i])
	}
	wg.Wait()
}

func parseGitVersion(body []byte) string {
	const key = `"gitVersion"`
	s := string(body)
	i := strings.Index(s, key)
	if i < 0 {
		return ""
	}
	rest := s[i+len(key):]
	start := strings.Index(rest, `"`)
	if start < 0 {
		return ""
	}
	rest = rest[start+1:]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// handleRemoveKubeconfig forgets a saved kubeconfig path. The file itself is
// left alone; the server's default kubeconfig cannot be removed.
func (s *Server) handleRemoveKubeconfig(w http.ResponseWriter, r *http.Request) {
	path := expandKubeconfigPath(r.URL.Query().Get("path"))
	if path == "" {
		httpError(w, fmt.Errorf("path is required"), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	if path == s.activeKubeconfigPath {
		s.mu.Unlock()
		httpError(w, fmt.Errorf("cannot remove the server's default kubeconfig; make another cluster the default first"), http.StatusConflict)
		return
	}
	if _, ok := s.knownKubeconfigPaths[path]; !ok {
		s.mu.Unlock()
		httpError(w, fmt.Errorf("kubeconfig %q is not saved", path), http.StatusNotFound)
		return
	}
	delete(s.knownKubeconfigPaths, path)
	s.persistKubeconfigStateLocked()
	s.mu.Unlock()
	s.clusters.forgetPath(path)
	s.log.Info("Removed saved kubeconfig", zap.String("kubeconfig", path))
	s.handleListClusters(w, r)
}
