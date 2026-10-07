package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/sardine-ai/go-remote-config/source"
)

const cacheTestYAML = `
str_map:
  a: "1"
  b: "2"
list: [x, y, z]
nested:
  outer:
    inner: v
  items: [1, 2]
sla:
  c1: {initial_timer_days: 1, extension_timer_days: 2}
  c2: {initial_timer_days: 3}
scalar: hello
num: 42
ratio: 0.5
flag: true
dur: 1s
when: 2024-01-02T03:04:05Z
numkeys: {1: a, 2: b}
nothing:
mixed: [1, a, true, 2.0, null]
floats: {a: 2.0, b: 0.25}
`

type cacheSLA struct {
	Initial int `yaml:"initial_timer_days"`
	Ext     int `yaml:"extension_timer_days"`
}

type cacheNamedMap map[string]string

func newFileClient(t testing.TB, content string) (*Client, *source.FileRepository, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	repo := &source.FileRepository{Name: "test", Path: path}
	if err := repo.Refresh(); err != nil {
		t.Fatal(err)
	}
	return &Client{Repository: repo}, repo, path
}

func rewrite(t testing.TB, repo *source.FileRepository, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := repo.Refresh(); err != nil {
		t.Fatal(err)
	}
}

func cacheLen(c *Client) int {
	n := 0
	c.cache.Range(func(_, _ interface{}) bool { n++; return true })
	return n
}

func safeErr(f func() error) (out string) {
	defer func() {
		if r := recover(); r != nil {
			out = fmt.Sprintf("panic: %v", r)
		}
	}()
	if err := f(); err != nil {
		return err.Error()
	}
	return ""
}

func TestGetConfigReusesCachedEntry(t *testing.T) {
	c, _, _ := newFileClient(t, cacheTestYAML)
	var a, b map[string]string
	if err := c.GetConfig("str_map", &a, nil); err != nil {
		t.Fatal(err)
	}
	key := cacheKey{"str_map", reflect.TypeOf(&a)}
	first, _ := c.cache.Load(key)
	if err := c.GetConfig("str_map", &b, nil); err != nil {
		t.Fatal(err)
	}
	second, _ := c.cache.Load(key)
	if first == nil || first != second {
		t.Fatal("second call should reuse the cached entry")
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("%v != %v", a, b)
	}
}

func TestGetConfigSeesRepositoryChanges(t *testing.T) {
	c, repo, path := newFileClient(t, cacheTestYAML)
	var m map[string]string
	if err := c.GetConfig("str_map", &m, nil); err != nil || m["a"] != "1" {
		t.Fatalf("%v %v", err, m)
	}

	// Changed externally: the next call rebuilds inline.
	rewrite(t, repo, path, "str_map: {a: changed}\n")
	m = nil
	if err := c.GetConfig("str_map", &m, nil); err != nil || !reflect.DeepEqual(m, map[string]string{"a": "changed"}) {
		t.Fatalf("%v %v", err, m)
	}

	// Refresh loop path: rebuildCache re-converts ahead of live calls.
	rewrite(t, repo, path, "str_map: {a: rebuilt}\n")
	c.rebuildCache()
	v, _ := c.cache.Load(cacheKey{"str_map", reflect.TypeOf(&m)})
	src, _ := repo.GetData("str_map")
	if v == nil || !sameSource(v.(*cacheEntry).src, src) {
		t.Fatal("rebuildCache should refresh the entry to the new source")
	}
	m = nil
	if err := c.GetConfig("str_map", &m, nil); err != nil || m["a"] != "rebuilt" {
		t.Fatalf("%v %v", err, m)
	}

	// Removed key.
	rewrite(t, repo, path, "other: 1\n")
	if err := c.GetConfig("str_map", &m, nil); err != ErrConfigNotFound {
		t.Fatalf("want ErrConfigNotFound, got %v", err)
	}
	c.rebuildCache()
	if cacheLen(c) != 0 {
		t.Fatal("rebuildCache should drop entries for removed keys")
	}
}

func TestGetConfigResultsAreIndependentCopies(t *testing.T) {
	c, _, _ := newFileClient(t, cacheTestYAML)
	type run struct {
		key string
		mk  func() interface{}
		mut func(interface{})
	}
	runs := []run{
		{"str_map", func() interface{} { return new(map[string]string) }, func(p interface{}) { m := *p.(*map[string]string); m["a"] = "X"; m["new"] = "X" }},
		{"str_map", func() interface{} { return new(cacheNamedMap) }, func(p interface{}) { m := *p.(*cacheNamedMap); m["a"] = "X" }},
		{"str_map", func() interface{} { return new(map[string]interface{}) }, func(p interface{}) { m := *p.(*map[string]interface{}); m["a"] = "X" }},
		{"list", func() interface{} { return new([]string) }, func(p interface{}) { s := *p.(*[]string); s[0] = "X" }},
		{"list", func() interface{} { return new([]interface{}) }, func(p interface{}) { s := *p.(*[]interface{}); s[0] = "X" }},
		{"nested", func() interface{} { return new(map[string]interface{}) }, func(p interface{}) {
			m := *p.(*map[string]interface{})
			m["outer"].(map[string]interface{})["inner"] = "X"
			m["items"].([]interface{})[0] = "X"
		}},
		{"sla", func() interface{} { return new(map[string]cacheSLA) }, func(p interface{}) { m := *p.(*map[string]cacheSLA); m["c1"] = cacheSLA{99, 99}; delete(m, "c2") }},
		{"sla", func() interface{} { return new(map[string]*cacheSLA) }, func(p interface{}) { m := *p.(*map[string]*cacheSLA); m["c1"].Initial = 99 }},
	}
	for _, r := range runs {
		first, again := r.mk(), r.mk()
		if err := c.GetConfig(r.key, first, nil); err != nil {
			t.Fatal(err)
		}
		before := r.mk()
		if err := c.GetConfig(r.key, before, nil); err != nil {
			t.Fatal(err)
		}
		r.mut(first)
		if err := c.GetConfig(r.key, again, nil); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(again, before) {
			t.Fatalf("%s/%T: mutation leaked into later calls: %#v vs %#v", r.key, first, again, before)
		}
	}
}

func TestGetConfigFallsBackToOriginalConversion(t *testing.T) {
	c, repo, _ := newFileClient(t, cacheTestYAML)

	// Non-empty target keeps YAML merge semantics and is not cached.
	merged := map[string]string{"keep": "me"}
	if err := c.GetConfig("str_map", &merged, nil); err != nil || merged["keep"] != "me" || merged["a"] != "1" {
		t.Fatalf("merge semantics lost: %v %v", err, merged)
	}
	if cacheLen(c) != 0 {
		t.Fatal("non-empty target must not use the cache")
	}

	// Conversion errors are not cached and match the original error and partial fill.
	cfg, _ := repo.GetData("str_map")
	var gotMap, wantMap map[string]bool
	gotErr := c.GetConfig("str_map", &gotMap, map[string]bool{"d": true})
	wantErr := convertConfig(cfg, &wantMap, map[string]bool{"d": true})
	if gotErr == nil || wantErr == nil || gotErr.Error() != wantErr.Error() || !reflect.DeepEqual(gotMap, wantMap) {
		t.Fatalf("error parity: %v / %v, %v / %v", gotErr, wantErr, gotMap, wantMap)
	}
	if cacheLen(c) != 0 {
		t.Fatal("failed conversions must not be cached")
	}

	// Non-pointer and nil-pointer targets.
	for _, target := range []interface{}{map[string]string{}, (*map[string]string)(nil), nil} {
		got := safeErr(func() error { return c.GetConfig("str_map", target, nil) })
		want := safeErr(func() error { return convertConfig(cfg, target, nil) })
		if got != want {
			t.Fatalf("%T: %q != %q", target, got, want)
		}
	}

	// Any Repository is cached; a refresh that publishes a new value is picked up.
	mock := newMockRepository()
	mock.data["m"] = map[string]interface{}{"a": "b"}
	mc := &Client{Repository: mock}
	var out map[string]string
	if err := mc.GetConfig("m", &out, nil); err != nil || out["a"] != "b" {
		t.Fatal(err, out)
	}
	if cacheLen(mc) != 1 {
		t.Fatal("custom repositories are cached too")
	}
	mock.data["m"] = map[string]interface{}{"a": "published"}
	out = nil
	if err := mc.GetConfig("m", &out, nil); err != nil || out["a"] != "published" {
		t.Fatalf("new value must be picked up: %v %v", err, out)
	}
}

func TestGetConfigEmptyNonNilMapKeepsIdentity(t *testing.T) {
	c, _, _ := newFileClient(t, cacheTestYAML)
	m := map[string]string{}
	alias := m
	if err := c.GetConfig("str_map", &m, nil); err != nil {
		t.Fatal(err)
	}
	if alias["a"] != "1" || len(alias) != 2 {
		t.Fatalf("existing empty map should be filled in place: %v", alias)
	}
}

func TestGetConfigConcurrentWithRefresh(t *testing.T) {
	c, repo, path := newFileClient(t, cacheTestYAML)
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				var m map[string]string
				var l []string
				var s map[string]cacheSLA
				if err := c.GetConfig("str_map", &m, nil); err != nil || len(m) == 0 {
					t.Error(err, m)
					return
				}
				m["a"] = "mutated"
				if err := c.GetConfig("list", &l, nil); err != nil {
					t.Error(err)
					return
				}
				if err := c.GetConfig("sla", &s, nil); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	for i := 0; i < 20; i++ {
		rewrite(t, repo, path, cacheTestYAML)
		c.rebuildCache()
	}
	close(stop)
	wg.Wait()
}

func benchmarkConfig(n int) string {
	s := "models:\n"
	for i := 0; i < n; i++ {
		s += fmt.Sprintf("  \"iPhone%d,%d\": \"iPhone %d Pro Max\"\n", i/10, i%10, i)
	}
	s += "sla:\n"
	for i := 0; i < n/2; i++ {
		s += fmt.Sprintf("  client-%d: {initial_timer_days: %d, extension_timer_days: 2}\n", i, i)
	}
	return s
}

func BenchmarkGetConfigMapStringString(b *testing.B) {
	c, _, _ := newFileClient(b, benchmarkConfig(200))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out map[string]string
		if err := c.GetConfig("models", &out, nil); err != nil || len(out) != 200 {
			b.Fatal(err, len(out))
		}
	}
}

func BenchmarkGetConfigStructMap(b *testing.B) {
	c, _, _ := newFileClient(b, benchmarkConfig(200))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out map[string]cacheSLA
		if err := c.GetConfig("sla", &out, nil); err != nil || len(out) != 100 {
			b.Fatal(err, len(out))
		}
	}
}

// NaN values are cached and served like the original conversion.
func TestGetConfigNaNValueIsCached(t *testing.T) {
	c, _, _ := newFileClient(t, "nan: {a: .nan, b: 1.5}\n")
	for call := 0; call < 2; call++ {
		var got map[string]float64
		if err := c.GetConfig("nan", &got, nil); err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got["b"] != 1.5 || got["a"] == got["a"] {
			t.Fatalf("call %d: got %v", call, got)
		}
	}
	if cacheLen(c) != 1 {
		t.Fatal("NaN value should be cached")
	}
}

// A conversion that panics while the refresh loop rebuilds an entry drops the
// entry instead of crashing the refresh goroutine.
func TestRebuildEntryRecoversFromPanic(t *testing.T) {
	c, _, _ := newFileClient(t, cacheTestYAML)
	key := cacheKey{"scalar", reflect.TypeOf((*error)(nil))} // yaml panics decoding a string into error
	c.cache.Store(key, &cacheEntry{src: "stale", val: nil})
	c.rebuildEntry(key, "hello")
	if _, ok := c.cache.Load(key); ok {
		t.Fatal("entry should be dropped after a failed rebuild")
	}
}

func TestSameSource(t *testing.T) {
	m1, m2 := map[string]interface{}{"a": 1}, map[string]interface{}{"a": 1}
	s1 := []interface{}{1}
	cases := []struct {
		name string
		a, b interface{}
		want bool
	}{
		{"same map", m1, m1, true},
		{"equal but distinct maps", m1, m2, false},
		{"same slice", s1, s1, true},
		{"equal scalars", "x", "x", true},
		{"different scalars", "x", "y", false},
		{"different types", 1, "1", false},
		{"nil nil", nil, nil, true},
		{"nil vs value", nil, 1, false},
		{"uncomparable struct", struct{ f []int }{}, struct{ f []int }{}, false},
	}
	for _, tc := range cases {
		if got := sameSource(tc.a, tc.b); got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

// A nil map key is cached and served like the original conversion.
func TestGetConfigNilMapKeyIsCached(t *testing.T) {
	c, repo, _ := newFileClient(t, "nullkey: {null: x, a: y}\n")
	cfg, _ := repo.GetData("nullkey")
	var want map[interface{}]interface{}
	if err := convertConfig(cfg, &want, nil); err != nil {
		t.Fatal(err)
	}
	for call := 0; call < 2; call++ {
		var got map[interface{}]interface{}
		if err := c.GetConfig("nullkey", &got, nil); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) || len(got) != 2 {
			t.Fatalf("call %d: got %v want %v", call, got, want)
		}
	}
	if cacheLen(c) != 1 {
		t.Fatal("nil map key value should be cached")
	}
}

// wrapper mimics a decorator such as tweakpoint's instrumentedRepository.
type wrapper struct{ source.Repository }

// Mirrors sardine-all's tweakpoint client: a struct with `any` fields and a
// map of unexported override structs, read through a wrapped WebRepository-style repo.
type tpOverride struct {
	Value        any  `yaml:"value"`
	InheritValue bool `yaml:"inherit_value"`
}

type tpConfig struct {
	Type          string                `yaml:"type"`
	DefaultValue  any                   `yaml:"default_value"`
	AllowedValues []string              `yaml:"allowed_values,omitempty"`
	Visibility    string                `yaml:"visibility"`
	Overrides     map[string]tpOverride `yaml:"overrides,omitempty"`
}

const tpYAML = `
feature_a:
  type: bool
  default_value: false
  visibility: internal
  overrides:
    client-1: {value: true, inherit_value: true}
    client-2: {value: false, inherit_value: false}
limits:
  type: map
  default_value: {a: 1, b: [x, y], c: {d: 2.5}}
  visibility: internal
  allowed_values: [one, two]
  overrides:
    client-1: {value: {a: 9, b: [z]}, inherit_value: true}
`

func TestGetConfigTweakpointShapeThroughWrapper(t *testing.T) {
	_, repo, path := newFileClient(t, tpYAML)
	c := &Client{Repository: wrapper{repo}}
	for _, key := range []string{"feature_a", "limits"} {
		cfg, _ := repo.GetData(key)
		var want tpConfig
		if err := convertConfig(cfg, &want, nil); err != nil {
			t.Fatal(err)
		}
		for call := 0; call < 3; call++ {
			var got tpConfig
			if err := c.GetConfig(key, &got, nil); err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("%s call %d: %v\n got  %#v\n want %#v", key, call, err, got, want)
			}
			// mutate everything reachable, including `any` payloads
			got.Overrides["client-1"] = tpOverride{Value: "X"}
			if m, ok := got.DefaultValue.(map[string]interface{}); ok {
				m["a"] = "X"
				m["b"].([]interface{})[0] = "X"
			}
			if len(got.AllowedValues) > 0 {
				got.AllowedValues[0] = "X"
			}
		}
	}
	if cacheLen(c) != 2 {
		t.Fatalf("wrapped repository should be cached, entries=%d", cacheLen(c))
	}

	// refresh through the wrapper is picked up
	rewrite(t, repo, path, "feature_a: {type: bool, default_value: true, visibility: x}\n")
	c.rebuildCache()
	var after tpConfig
	if err := c.GetConfig("feature_a", &after, nil); err != nil || after.DefaultValue != true || len(after.Overrides) != 0 {
		t.Fatalf("%v %#v", err, after)
	}
}

func BenchmarkGetConfigTweakpointShape(b *testing.B) {
	_, repo, _ := newFileClient(b, tpYAML)
	c := &Client{Repository: wrapper{repo}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out tpConfig
		if err := c.GetConfig("feature_a", &out, nil); err != nil || len(out.Overrides) != 2 {
			b.Fatal(err)
		}
	}
}

// End to end in the shape tweakpoint uses: a WebRepository behind a decorator,
// refreshed by the client's background loop while GetConfig is served.
func TestGetConfigWebRepositoryBehindDecoratorWithRefreshLoop(t *testing.T) {
	var mu sync.Mutex
	body := tpYAML
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	web := &source.WebRepository{Name: "web", URL: u}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, err := NewClientWithOptions(ctx, wrapper{web}, 10*time.Millisecond, ClientOptions{SetAsDefault: false})
	if err != nil {
		t.Fatal(err)
	}

	var cfg tpConfig
	if err := c.GetConfig("feature_a", &cfg, nil); err != nil || cfg.DefaultValue != false || len(cfg.Overrides) != 2 {
		t.Fatalf("%v %#v", err, cfg)
	}
	key := cacheKey{"feature_a", reflect.TypeOf(&cfg)}
	if _, ok := c.cache.Load(key); !ok {
		t.Fatal("expected the entry to be cached")
	}

	mu.Lock()
	body = "feature_a: {type: bool, default_value: true, visibility: x, overrides: {client-9: {value: 1, inherit_value: true}}}\n"
	mu.Unlock()

	// The refresh loop alone (no GetConfig calls) must rebuild the entry.
	deadline := time.Now().Add(5 * time.Second)
	for {
		src, _ := web.GetData("feature_a")
		if v, ok := c.cache.Load(key); ok && sameSource(v.(*cacheEntry).src, src) && v.(*cacheEntry).val.(tpConfig).DefaultValue == true {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("refresh loop did not rebuild the cached entry")
		}
		time.Sleep(5 * time.Millisecond)
	}
	var after tpConfig
	if err := c.GetConfig("feature_a", &after, nil); err != nil || after.DefaultValue != true || len(after.Overrides) != 1 {
		t.Fatalf("%v %#v", err, after)
	}
	if allocs := testing.AllocsPerRun(100, func() {
		var out tpConfig
		_ = c.GetConfig("feature_a", &out, nil)
	}); allocs > 40 {
		t.Fatalf("expected the cached path, got %.0f allocs per call", allocs)
	}
}
