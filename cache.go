package levelcache

import (
	"crypto/md5"
)

const (
	version     int = 1000
	bucketLimit int = 256
)

type Hash [md5.Size]byte

type DevConf struct {
	Name     string
	Dir      string
	Capacity int
}

type Config struct {
	MetaDir        string
	ActionParallel int
	AuxFactory     AuxFactory
}

type Cache struct {
	conf    Config
	meta    *meta
	devices []*device
}

type Auxiliary interface {
	Add(key Hash, auxItem interface{})
	Get(key Hash) interface{}
	Del(key Hash)
	Load(path string)
	Dump(path string)
}

type AuxFactory func(idx int) Auxiliary

type Matcher func(aux Auxiliary) []Hash

func NewCache(conf Config, devices []DevConf) *Cache {
	cache := &Cache{
		conf:    conf,
		meta:    newMeta(conf.MetaDir, conf.AuxFactory),
		devices: make([]*device, len(devices))}

	for lv, devConf := range devices {
		cache.devices[lv] = newDevice(lv, devConf)
	}

	return cache
}

func (c *Cache) Close() {
	for _, d := range c.devices {
		d.close()
	}
}

func (c *Cache) Dump() {
	c.meta.dump(c.conf.ActionParallel)
	for _, d := range c.devices {
		d.dump(c.conf.ActionParallel)
	}
}

func (c *Cache) Get(key Hash, start int, end int) (dataList [][]byte, missSegments [][2]int) {
	item := c.meta.get(key)
	if item == nil {
		return nil, nil
	}

	if end == -1 {
		end = int(item.Size)
	}

	startSeg := uint32(start / int(item.SegSize))
	endSeg := uint32(end / int(item.SegSize))

	dataList = make([][]byte, 0)
	missSegments = make([][2]int, 0)

	for seg := startSeg; seg <= endSeg; seg++ {
		found := false
		for _, d := range c.devices {
			if tmp := d.get(key, seg); tmp != nil {
				dataList = append(dataList, tmp)
				found = true
				break
			}
		}

		if !found {
			segment := [2]int{
				int(seg * item.SegSize),
				int(seg*item.SegSize + seg)}
			missSegments = append(missSegments, segment)
		}
	}

	return dataList, missSegments
}

func (c *Cache) AddItem(key Hash, expire, size int64, auxData interface{}) {
	const maxSegSize uint32 = 1024 * 1024 * 64
	const minSegSize uint32 = 1024 * 1024
	const defaultSegCount int64 = 1024

	segSize := uint32(size / defaultSegCount)
	if segSize < minSegSize {
		segSize = minSegSize
	}
	if segSize > maxSegSize {
		segSize = maxSegSize
	}

	item := &item{
		Expire:   expire,
		Size:     size,
		SegSize:  segSize,
		Segments: make([]uint32, 0)}

	c.meta.addItem(key, item, auxData)
}

func (c *Cache) AddSegment(key Hash, start int, data []byte) {
	c.meta.addSegment(key, start, start+len(data), func(segIndex uint32) {
		c.devices[0].add(key, segIndex, data)
	})

}

func (c *Cache) Del(k Hash) {
	c.meta.del(k)
	for _, d := range c.devices {
		d.del(k)
	}
}

func (c *Cache) DelBatch(m Matcher) {
	c.meta.delBatch(c.conf.ActionParallel, m)
}
