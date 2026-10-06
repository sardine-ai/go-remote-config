package client

import (
	"maps"
	"reflect"
	"slices"

	"github.com/mohae/deepcopy"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// GetConfig conversions are cached per (name, type, source value).
// Only the refresh goroutine builds entries; on a miss GetConfig registers the
// key and uses the original YAML path, as it does whenever the cache could
// differ from it.
// Assumes GetData values are never mutated in place; see source.Repository.

type cacheKey struct {
	name string
	typ  reflect.Type // pointer type passed to GetConfig
}

// cacheEntry is immutable once stored.
type cacheEntry struct {
	src interface{} // source value the entry was built from
	val interface{} // converted value; never handed out directly
}

// sameSource reports whether a and b are the same source value.
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

// buildEntry runs the YAML conversion once; false if it must not be cached.
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

// copiesFaithfully reports whether deepcopy reproduces val exactly.
func copiesFaithfully(val interface{}) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return reflect.DeepEqual(deepcopy.Copy(val), val)
}

// lookup returns the cached entry for (name, type) if it matches src.
// On a miss it registers the key for the refresh goroutine and returns false.
func (c *Client) lookup(name string, src interface{}, ptrType reflect.Type) (*cacheEntry, bool) {
	key := cacheKey{name, ptrType}
	if v, ok := c.cache.Load(key); ok {
		if e := v.(*cacheEntry); sameSource(e.src, src) {
			return e, true
		}
	}
	if _, ok := c.wanted.Load(key); !ok {
		if _, loaded := c.wanted.LoadOrStore(key, struct{}{}); !loaded {
			select {
			case c.rebuildCh <- struct{}{}:
			default:
			}
		}
	}
	return nil, false
}

// serveFromCache fills data from the cache; false means use the original path.
func (c *Client) serveFromCache(name string, config interface{}, data interface{}) bool {
	dv := reflect.ValueOf(data)
	if dv.Kind() != reflect.Ptr || dv.IsNil() {
		return false
	}
	target := dv.Elem()
	// YAML merges into non-empty targets; leave them to the original path.
	if !target.IsZero() && !isEmptyContainer(target) {
		return false
	}
	e, ok := c.lookup(name, config, dv.Type())
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
	// fast paths for exact common types; others use deepcopy
	switch v := val.(type) {
	case map[string]string:
		if t, ok := target.Addr().Interface().(*map[string]string); ok {
			if *t == nil || v == nil {
				*t = maps.Clone(v)
			} else {
				maps.Copy(*t, v) // empty non-nil target: fill in place, as yaml does
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
		// empty non-nil target: fill in place, as yaml does
		for it := cp.MapRange(); it.Next(); {
			target.SetMapIndex(it.Key(), it.Value())
		}
		return
	}
	target.Set(cp)
}

// rebuildCache re-converts wanted entries whose source changed.
// It runs only on the refresh goroutine, the single writer of c.cache.
func (c *Client) rebuildCache() {
	c.wanted.Range(func(k, _ interface{}) bool {
		key := k.(cacheKey)
		src, ok := c.Repository.GetData(key.name)
		if !ok {
			c.cache.Delete(key)
			return true
		}
		if v, ok := c.cache.Load(key); ok && sameSource(v.(*cacheEntry).src, src) {
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
