package dashboard

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/kubepilot/kubepilot/pkg/k8s"
)

// fakeAPIServer answers just enough of the Kubernetes API for these tests:
// /version and a namespace list containing one namespace named after it.
func fakeAPIServer(t *testing.T, name string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/version":
			fmt.Fprintf(w, `{"major":"1","minor":"31","gitVersion":"v1.31.0+%s"}`, name)
		case "/api/v1/namespaces":
			fmt.Fprintf(w, `{"kind":"NamespaceList","apiVersion":"v1","metadata":{},"items":[{"metadata":{"name":"ns-%s"},"status":{"phase":"Active"}}]}`, name)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func writeKubeconfig(t *testing.T, dir, file, current string, servers map[string]string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("apiVersion: v1\nkind: Config\ncurrent-context: " + current + "\nclusters:\n")
	for ctx, url := range servers {
		fmt.Fprintf(&b, "- name: %s\n  cluster:\n    server: %s\n", ctx, url)
	}
	b.WriteString("users:\n- name: u\n  user:\n    token: t\ncontexts:\n")
	for ctx := range servers {
		fmt.Fprintf(&b, "- name: %s\n  context:\n    cluster: %s\n    user: u\n", ctx, ctx)
	}
	p := filepath.Join(dir, file)
	if err := os.WriteFile(p, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

type testEnv struct {
	handler http.Handler
	prod    string // path of the default kubeconfig
	lab     string // path of the second kubeconfig
	dir     string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir) // keep kubeconfigs.json out of the real home
	prod := writeKubeconfig(t, dir, "prod.yaml", "default", map[string]string{"default": fakeAPIServer(t, "prod").URL})
	lab := writeKubeconfig(t, dir, "lab.yaml", "east", map[string]string{
		"east": fakeAPIServer(t, "east").URL,
		"west": fakeAPIServer(t, "west").URL,
	})
	client, err := k8s.NewClient(prod)
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(Config{K8sClient: client, KubeconfigPath: prod, EnableKubeconfigMutationEndpoints: true}, zap.NewNop())
	srv.knownKubeconfigPaths[lab] = struct{}{}
	return &testEnv{handler: srv.Handler(), prod: prod, lab: lab, dir: dir}
}

func (e *testEnv) do(t *testing.T, method, path, cluster, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if cluster != "" {
		req.Header.Set(ClusterHeader, cluster)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

type clusterList struct {
	DefaultID string        `json:"default_id"`
	Clusters  []ClusterInfo `json:"clusters"`
	AddedIDs  []string      `json:"added_ids"`
}

func (e *testEnv) clusters(t *testing.T, query string) clusterList {
	t.Helper()
	rec := e.do(t, http.MethodGet, "/api/v1/clusters"+query, "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list clusters: %d %s", rec.Code, rec.Body)
	}
	var out clusterList
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func byName(l clusterList) map[string]ClusterInfo {
	m := map[string]ClusterInfo{}
	for _, c := range l.Clusters {
		m[c.Name] = c
	}
	return m
}

func TestClusterListExpandsContextsAndNamesThem(t *testing.T) {
	env := newTestEnv(t)
	list := env.clusters(t, "?probe=true")
	got := byName(list)
	if len(got) != 3 {
		t.Fatalf("clusters = %+v", list.Clusters)
	}
	// k3s-style "default" contexts take the file name.
	prod, ok := got["prod"]
	if !ok || !prod.Default || list.DefaultID != prod.ID {
		t.Fatalf("prod should be the default: %+v (default %s)", got, list.DefaultID)
	}
	for _, name := range []string{"east", "west"} {
		c := got[name]
		if c.Default || c.Reachable == nil || !*c.Reachable || c.Version != "v1.31.0+"+name {
			t.Errorf("%s = %+v", name, c)
		}
	}
	if strings.Contains(string(mustJSON(t, list)), env.dir) == false {
		t.Errorf("kubeconfig paths should be listed for management")
	}
}

func TestRequestsRouteToTheirSelectedCluster(t *testing.T) {
	env := newTestEnv(t)
	got := byName(env.clusters(t, ""))

	for cluster, want := range map[string]string{
		"":             "ns-prod",
		got["east"].ID: "ns-east",
		got["west"].ID: "ns-west",
		got["prod"].ID: "ns-prod",
	} {
		rec := env.do(t, http.MethodGet, "/api/v1/namespaces", cluster, "")
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("cluster %q: %d %s, want %s", cluster, rec.Code, rec.Body, want)
		}
	}
	// The query parameter works where a header cannot be set.
	rec := env.do(t, http.MethodGet, "/api/v1/namespaces?cluster="+got["west"].ID, "", "")
	if !strings.Contains(rec.Body.String(), "ns-west") {
		t.Errorf("query selection: %s", rec.Body)
	}
	// Selecting a cluster never moves the default for everyone else.
	if rec := env.do(t, http.MethodGet, "/api/v1/namespaces", "", ""); !strings.Contains(rec.Body.String(), "ns-prod") {
		t.Errorf("default moved: %s", rec.Body)
	}
	if rec := env.do(t, http.MethodGet, "/api/v1/namespaces", "nope", ""); rec.Code != http.StatusConflict {
		t.Errorf("unknown cluster: %d %s", rec.Code, rec.Body)
	}
}

func TestUploadWithoutActivateKeepsDefaultAndReturnsIDs(t *testing.T) {
	env := newTestEnv(t)
	before := env.clusters(t, "").DefaultID

	src := writeKubeconfig(t, t.TempDir(), "edge.yaml", "edge", map[string]string{"edge": fakeAPIServer(t, "edge").URL})
	raw, _ := os.ReadFile(src)
	body := fmt.Sprintf(`{"name":"edge.yaml","content_base64":%q,"activate":false}`, base64.StdEncoding.EncodeToString(raw))
	rec := env.do(t, http.MethodPost, "/api/v1/clusters/kubeconfigs/base64", "", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body)
	}
	var added clusterList
	_ = json.Unmarshal(rec.Body.Bytes(), &added)
	if len(added.AddedIDs) != 1 || added.DefaultID != before {
		t.Fatalf("added=%v default %s→%s", added.AddedIDs, before, added.DefaultID)
	}
	if rec := env.do(t, http.MethodGet, "/api/v1/namespaces", added.AddedIDs[0], ""); !strings.Contains(rec.Body.String(), "ns-edge") {
		t.Errorf("new cluster not selectable: %s", rec.Body)
	}

	bad := fmt.Sprintf(`{"name":"bad.yaml","content_base64":%q,"activate":false}`, base64.StdEncoding.EncodeToString([]byte("not: [yaml")))
	if rec := env.do(t, http.MethodPost, "/api/v1/clusters/kubeconfigs/base64", "", bad); rec.Code != http.StatusBadRequest {
		t.Errorf("invalid kubeconfig accepted: %d", rec.Code)
	}
}

func TestSetDefaultAndRemoveKubeconfig(t *testing.T) {
	env := newTestEnv(t)
	got := byName(env.clusters(t, ""))

	if rec := env.do(t, http.MethodDelete, "/api/v1/clusters/kubeconfigs?path="+env.prod, "", ""); rec.Code != http.StatusConflict {
		t.Errorf("removing the default kubeconfig: %d", rec.Code)
	}
	rec := env.do(t, http.MethodPost, "/api/v1/clusters/default", "", fmt.Sprintf(`{"id":%q}`, got["west"].ID))
	if rec.Code != http.StatusOK {
		t.Fatalf("set default: %d %s", rec.Code, rec.Body)
	}
	if rec := env.do(t, http.MethodGet, "/api/v1/namespaces", "", ""); !strings.Contains(rec.Body.String(), "ns-west") {
		t.Errorf("default not moved to west: %s", rec.Body)
	}
	if rec := env.do(t, http.MethodDelete, "/api/v1/clusters/kubeconfigs?path="+env.prod, "", ""); rec.Code != http.StatusOK {
		t.Fatalf("remove prod: %d %s", rec.Code, rec.Body)
	}
	if _, ok := byName(env.clusters(t, ""))["prod"]; ok {
		t.Error("prod still listed after removal")
	}
	if rec := env.do(t, http.MethodGet, "/api/v1/namespaces", got["prod"].ID, ""); rec.Code != http.StatusConflict {
		t.Errorf("removed cluster still selectable: %d", rec.Code)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
