package cache

import (
	"time"

	"github.com/bluele/gcache"
	"github.com/nspcc-dev/neofs-sdk-go/object"
	"github.com/nspcc-dev/neofs-sdk-go/object/address"
)

// ObjectsCache provides lru cache for objects.
type ObjectsCache struct {
	cache gcache.Cache
}

const (
	// DefaultObjectsCacheLifetime is a default lifetime of entries in objects' cache.
	DefaultObjectsCacheLifetime = time.Minute * 5
	// DefaultObjectsCacheSize is a default maximum number of entries in objects' cache.
	DefaultObjectsCacheSize = 1e6
)

// DefaultObjectsConfig returns new default cache expiration values.
func DefaultObjectsConfig() *Config {
	return &Config{Size: DefaultObjectsCacheSize, Lifetime: DefaultObjectsCacheLifetime}
}

// New creates an object of ObjectHeadersCache.
func New(config *Config) *ObjectsCache {
	gc := gcache.New(config.Size).LRU().Expiration(config.Lifetime).Build()
	return &ObjectsCache{cache: gc}
}

// Get returns a cached object.
func (o *ObjectsCache) Get(address *address.Address) *object.Object {
	entry, err := o.cache.Get(address.String())
	if err != nil {
		return nil
	}

	result, ok := entry.(object.Object)
	if !ok {
		return nil
	}

	return &result
}

// Put puts an object to cache.
func (o *ObjectsCache) Put(obj object.Object) error {
	cnrID, _ := obj.ContainerID()
	objID, _ := obj.ID()
	return o.cache.Set(cnrID.String()+"/"+objID.String(), obj)
}

// Delete deletes an object from cache.
func (o *ObjectsCache) Delete(address *address.Address) bool {
	return o.cache.Remove(address.String())
}
