package main

import (
	"crypto/md5"
	"fmt"
	cache "github.com/ghostzch/levelcache"
	"hash/crc32"
	"os"
	"time"
)

type httpAuxData struct {
	fileType uint32
}

type httpAux struct {
	datas map[cache.Hash]httpAuxData
}

func (aux *httpAux) Add(key cache.Hash, auxItem interface{}) {
	aux.datas[key] = auxItem.(httpAuxData)
}

func (aux *httpAux) Get(key cache.Hash) interface{} {
	data, _ := aux.datas[key]
	return data
}

func (aux *httpAux) Del(key cache.Hash) {
	delete(aux.datas, key)
}

func (aux *httpAux) Load(path string) {
	return
}

func (aux *httpAux) Dump(path string) {
	return
}

func NewHttpAux(idx int) cache.Auxiliary {
	return &httpAux{datas: make(map[cache.Hash]httpAuxData)}
}

func main() {
	os.MkdirAll("/tmp/cache/meta/", os.ModePerm)
	os.MkdirAll("/tmp/cache/hdd/", os.ModePerm)
	os.MkdirAll("/tmp/cache/ssd/", os.ModePerm)
	os.MkdirAll("/tmp/cache/mem/", os.ModePerm)

	conf := cache.Config{
		MetaDir:        "/tmp/cache/meta/",
		ActionParallel: 4,
		AuxFactory:     NewHttpAux}

	devices := [3]cache.DevConf{
		cache.DevConf{
			Name:     "hdd",
			Dir:      "/tmp/cache/hdd/",
			Capacity: 1000 * 1024 * 1024},
		cache.DevConf{
			Name:     "ssd",
			Dir:      "/tmp/cache/ssd/",
			Capacity: 100 * 1024 * 1024},
		cache.DevConf{
			Name:     "mem",
			Dir:      "/tmp/cache/mem/",
			Capacity: 10 * 1024 * 1024},
	}

	c := cache.NewCache(conf, devices[:])
	defer c.Close()
	defer os.RemoveAll("/tmp/cache")

	go func() {
		time.Sleep(time.Minute)
		c.Dump()
	}()

	rawKey := []byte("http://www.test.com/123/456/1.jpg")
	key := md5.Sum(rawKey)

	fmt.Println(c.Get(key, 0, -1)) // nil, nil

	data := []byte("this is 1.jpg")
	jpg := crc32.ChecksumIEEE([]byte("jpg"))
	c.AddItem(key, time.Now().Unix()+3600, int64(len(data)), &httpAuxData{fileType: jpg})
	c.AddSegment(key, 0, data)

	fmt.Println(c.Get(key, 0, -1))

	c.DelBatch(func(aux cache.Auxiliary) []cache.Hash {
		keys := make([]cache.Hash, 0)
		for k, v := range aux.(*httpAux).datas {
			if v.fileType == jpg {
				keys = append(keys, k)
			}
		}
		return keys
	})
	fmt.Println(c.Get(key, 0, -1))

}
