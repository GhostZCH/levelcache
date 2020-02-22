package levelcache

import (
	"encoding/gob"
	"fmt"
	"os"
	"sync"
)

type segKey struct {
	Key   Hash
	Index uint32
}

type segValue struct {
	Block int64
	Off   int64
	Size  int64
}

type devBucket struct {
	metaPath string
	lock     sync.RWMutex
	segments map[Hash]map[uint32]*segValue
	blockMap map[int64][]segKey
}

type device struct {
	conf    *DevConf
	buckets [bucketLimit]*devBucket
	store   *store
}

func newDevice(level int, conf DevConf) *device {
	d := &device{conf: &conf, store: newStore(conf.Dir, int64(conf.Capacity))}
	for i := 0; i < bucketLimit; i++ {
		d.buckets[i] = newDevBucket(conf.Dir, i)
	}
	return d
}

func newDevBucket(dir string, idx int) *devBucket {
	metaPath := fmt.Sprintf("%s/%d-%02d.bkt", dir, version, idx)

	bkt := &devBucket{
		metaPath: metaPath,
		segments: make(map[Hash]map[uint32]*segValue),
		blockMap: make(map[int64][]segKey)}

	if meta, err := os.Open(metaPath); err != nil {
		gob.NewDecoder(meta).Decode(&bkt.segments)
		meta.Close()
		for k, v := range bkt.segments {
			for i, s := range v {
				if _, ok := bkt.blockMap[s.Block]; ok {
					bkt.blockMap[s.Block] = make([]segKey, 0)
				}
				bkt.blockMap[s.Block] = append(bkt.blockMap[s.Block], segKey{Key: k, Index: i})
			}
		}
	} else if !os.IsNotExist(err) {
		panic(err)
	}

	return bkt
}

func (d *device) close() {
	d.store.close()
	d.dump(8)
}

func (d *device) dump(parallel int) {
	d.foreachBucket(parallel, func(b *devBucket) {
		b.lock.Lock()
		defer b.lock.Unlock()
		safeDump(b.metaPath, b.segments)
	})
}

func (d *device) foreachBucket(parallel int, handler func(b *devBucket)) {
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
				b := d.buckets[idx]
				handler(b)
			}
		}()
	}
	wg.Wait()
}

func (d *device) get(key Hash, seg uint32) []byte {
	b := d.getBucket(key)
	b.lock.RLock()
	defer b.lock.RUnlock()
	if s, ok := b.segments[key]; ok {
		if v, ok := s[seg]; ok {
			return d.store.get(v)
		}
	}
	return nil
}

func (d *device) add(k Hash, seg uint32, data []byte) {
	b := d.getBucket(k)
	b.lock.Lock()
	defer b.lock.Unlock()

	block, off := d.store.add(data)

	if _, ok := b.segments[k]; !ok {
		b.segments[k] = make(map[uint32]*segValue)
	}

	b.segments[k][seg] = &segValue{
		Block: block,
		Off:   off,
		Size:  int64(len(data))}
}

func (d *device) del(k Hash) {
	b := d.getBucket(k)
	b.lock.Lock()
	defer b.lock.Unlock()
	delete(b.segments, k)
}

func (d *device) delBlock(block int64) {
	for i := 0; i < bucketLimit; i++ {
		func() {
			b := d.buckets[i]
			b.lock.Lock()
			defer b.lock.Unlock()
			for _, k := range b.blockMap[block] {
				if segs, ok := b.segments[k.Key]; ok {
					delete(segs, k.Index)
					if len(segs) == 0 {
						delete(b.segments, k.Key)
					}
				}
			}
		}()
	}
}

func (d *device) getBucket(k Hash) *devBucket {
	return d.buckets[k[0]]
}
