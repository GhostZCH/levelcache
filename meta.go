package levelcache

import (
	"encoding/gob"
	"fmt"
	"os"
	"sync"
)

type item struct {
	Expire   int64
	SegSize  uint32
	Segments []uint32
}

type metaBucket struct {
	idx      int
	itemPath string
	auxPath  string
	lock     sync.RWMutex
	items    map[Hash]*item
	aux      Auxiliary
}

type meta struct {
	dir     string
	buckets [bucketLimit]*metaBucket
}

func newMeta(dir string, factory AuxFactory) *meta {
	m := &meta{dir: dir}

	for i := 0; i < bucketLimit; i++ {
		m.buckets[i] = newMetaBucket(dir, i, factory(i))
	}

	return m
}

func newMetaBucket(dir string, idx int, aux Auxiliary) *metaBucket {
	b := &metaBucket{
		itemPath: fmt.Sprintf("%s/%d-%d.item", dir, version, idx),
		auxPath:  fmt.Sprintf("%s/%d-%d.aux", dir, version, idx),
		items:    make(map[Hash]*item),
		aux:      aux}

	aux.Load(b.auxPath)

	if meta, err := os.Open(b.itemPath); err != nil {
		gob.NewDecoder(meta).Decode(&b.items)
		meta.Close()
	} else if os.IsNotExist(err) {
		panic(err)
	}

	return b
}

func (m *meta) add(k Hash, item *item, auxData interface{}) {
	b := m.getBucket(k)
	b.lock.Lock()
	defer b.lock.Unlock()

	b.items[k] = item
	b.aux.Add(k, auxData)
}

func (m *meta) del(k Hash) {
	b := m.getBucket(k)
	b.lock.Lock()
	defer b.lock.Unlock()

	delete(b.items, k)
	b.aux.Del(k)
}

func (m *meta) delBatch(parallel int, macher Matcher) {
	m.foreachBucket(parallel, func(b *metaBucket) {
		keys := func() []Hash {
			b.lock.RLock()
			defer b.lock.RUnlock()
			return macher(b.aux)
		}()

		func() {
			b.lock.Lock()
			defer b.lock.Unlock()
			for _, k := range keys {
				delete(b.items, k)
				b.aux.Del(k)
			}
		}()
	})
}

func (m *meta) dump(parallel int) {
	m.foreachBucket(parallel, func(b *metaBucket) {
		b.lock.RLock()
		defer b.lock.RUnlock()
		safeDump(b.itemPath, b.items)
		b.aux.Dump(b.auxPath)
	})
}

func (m *meta) foreachBucket(parallel int, handler func(b *metaBucket)) {
	buckets := make(chan int, bucketLimit+parallel)
	for i := 0; i < bucketLimit+parallel; i++ {
		buckets <- i
	}

	var wg sync.WaitGroup
	wg.Add(parallel)
	for i := 0; i < parallel; i++ {
		go func() {
			defer wg.Done()
			for idx := range buckets {
				if idx >= bucketLimit {
					return
				}
				b := m.buckets[idx]
				handler(b)
			}
		}()
	}
	wg.Wait()
}

func (m *meta) getBucket(k Hash) *metaBucket {
	return m.buckets[k[0]]
}
