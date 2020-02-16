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
	for _, d := range c.devices {
		d.dump(c.conf.ActionParallel)
	}
}

func (c *Cache) Get(key Hash, start int, end int) (data [][]byte, miss []uint32) {
	// item := c.meta
	return nil, nil
}

// func (c *Cache) Add(key Hash, seg int, data []byte) {
// 	return c.devices[len(c.devices)-1].add(item)
// }

func (c *Cache) Del(k Hash) {
	c.meta.del(k)
}

func (c *Cache) DelBatch(m Matcher) {
	c.meta.delBatch(c.conf.ActionParallel, m)
}
