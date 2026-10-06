package client

import (
	"maps"
	"reflect"
	"slices"

	"github.com/mohae/deepcopy"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// GetConfig converts the repository's decoded value into the caller's type
// via a YAML round-trip. That conversion is deterministic for a given
// (config name, target type, source value), so it is done once and cached;
// later calls only copy the cached result. The cache is rebuilt by the
// background refresh loop after each successful refresh.
//
// Backward compatibility is the priority: whenever the cached path could
// behave differently from the original per-call conversion (conversion errors,
// non-pointer or non-empty targets, types that cannot be copied faithfully)
// GetConfig runs the original YAML code instead.
//
// The cache relies on the source.Repository contract that GetData values are
// never mutated in place: a refresh publishes a new map/slice, which sameSource
// detects by identity.

type cacheKey struct {
	name string
	typ  reflect.Type // type of the pointer passed to GetConfig
}

// cacheEntry is immutable once stored.
type cacheEntry struct {
	src interface{} // repository value the entry was built from
	val interface{} // converted value of the pointed-to type; never handed out directly
}

// sameSource reports whether a and b are the same repository value: the same
// map/slice object, or equal scalars.
func sameSource(a, b interface{}) (same bool) {
	defer func() {
		if recover() != nil {
			same = false
		}
	}()
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	if !va.IsValid() || !vb.IsValid() {
		return !va.IsValid() && !vb.IsValid()
	}
	if va.Type() != vb.Type() {
		return false
	}
	switch va.Kind() {
	case reflect.Map, reflect.Slice:
		return va.Pointer() == vb.Pointer() && va.Len() == vb.Len()
	}
	return a == b
}

// buildEntry runs the original conversion once. It returns false when the
// result must not be cached: any conversion error, or a value deepcopy cannot
// reproduce exactly (e.g. unexported fields, NaN).
func buildEntry(src interface{}, ptrType reflect.Type) (*cacheEntry, bool) {
	ptr := reflect.New(ptrType.Elem())
	marshal, err := yaml.Marshal(src)
	if err != nil {
		return nil, false
	}
	if err := yaml.Unmarshal(marshal, ptr.Interface()); err != nil {
		return nil, false
	}
	val := ptr.Elem().Interface()
	if !copiesFaithfully(val) {
		return nil, false
	}
	return &cacheEntry{src: src, val: val}, true
}

// copiesFaithfully reports whether deepcopy reproduces val exactly. deepcopy
// panics on some values (e.g. a nil map key), which also means "no".
func copiesFaithfully(val interface{}) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return reflect.DeepEqual(deepcopy.Copy(val), val)
}

// getOrBuild returns the entry for (name, type) for the given source value,
// rebuilding it when the repository value changed.
func (c *Client) getOrBuild(name string, src interface{}, ptrType reflect.Type) (*cacheEntry, bool) {
	key := cacheKey{name, ptrType}
	if v, ok := c.cache.Load(key); ok {
		if e := v.(*cacheEntry); sameSource(e.src, src) {
			return e, true
		}
	}
	e, ok := buildEntry(src, ptrType)
	if !ok {
		c.cache.Delete(key)
		return nil, false
	}
	c.cache.Store(key, e)
	return e, true
}

// serveFromCache fills data from the cache. It returns false when the call
// must take the original conversion path.
func (c *Client) serveFromCache(name string, config interface{}, data interface{}) bool {
	dv := reflect.ValueOf(data)
	if dv.Kind() != reflect.Ptr || dv.IsNil() {
		return false
	}
	target := dv.Elem()
	// YAML merges into a non-empty map and keeps other existing state; leave
	// every target that is not zero or an empty map/slice to the original path.
	if !target.IsZero() && !isEmptyContainer(target) {
		return false
	}
	e, ok := c.getOrBuild(name, config, dv.Type())
	if !ok {
		return false
	}
	copyInto(target, e.val)
	return true
}

func isEmptyContainer(v reflect.Value) bool {
	return (v.Kind() == reflect.Map || v.Kind() == reflect.Slice) && v.Len() == 0
}

// copyInto stores an independent copy of val in target.
func copyInto(target reflect.Value, val interface{}) {
	// Hot types without reflection. Exact types only; named types use deepcopy.
	switch v := val.(type) {
	case map[string]string:
		if t, ok := target.Addr().Interface().(*map[string]string); ok {
			if *t == nil || v == nil {
				*t = maps.Clone(v)
			} else {
				maps.Copy(*t, v) // empty non-nil target: fill it, as yaml does
			}
			return
		}
	case []string:
		if t, ok := target.Addr().Interface().(*[]string); ok {
			*t = slices.Clone(v)
			return
		}
	}

	if val == nil {
		target.Set(reflect.Zero(target.Type()))
		return
	}
	cp := reflect.ValueOf(deepcopy.Copy(val))
	if target.Kind() == reflect.Map && !target.IsNil() && !cp.IsNil() {
		// Empty non-nil target map: fill it in place, as yaml does.
		for it := cp.MapRange(); it.Next(); {
			target.SetMapIndex(it.Key(), it.Value())
		}
		return
	}
	target.Set(cp)
}

// rebuildCache re-converts every cached entry whose repository value changed.
// It runs on the refresh goroutine so live GetConfig calls rarely do it.
func (c *Client) rebuildCache() {
	c.cache.Range(func(k, v interface{}) bool {
		key := k.(cacheKey)
		src, ok := c.Repository.GetData(key.name)
		if !ok {
			c.cache.Delete(key)
			return true
		}
		if sameSource(v.(*cacheEntry).src, src) {
			return true
		}
		c.rebuildEntry(key, src)
		return true
	})
}

func (c *Client) rebuildEntry(key cacheKey, src interface{}) {
	defer func() {
		if r := recover(); r != nil {
			logrus.WithField("config", key.name).Errorf("panic rebuilding config cache: %v", r)
			c.cache.Delete(key)
		}
	}()
	if e, ok := buildEntry(src, key.typ); ok {
		c.cache.Store(key, e)
	} else {
		c.cache.Delete(key)
	}
}
