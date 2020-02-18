package main

import (
	"crypto/md5"
	"fmt"
	cache "github.com/ghostzch/levelcache"
	"hash/crc32"
	"os"
	"regexp"
	"time"
)

type httpAuxData struct {
	headerLen uint32
	fileType  uint32
	expireCRC uint32
	etagCRC   uint32
	userIDCRC uint32
	rawKey    []byte
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
	os.RemoveAll("/tmp/cache")
	os.MkdirAll("/tmp/cache/meta/", os.ModeDir|os.ModePerm)
	os.MkdirAll("/tmp/cache/hdd/", os.ModeDir|os.ModePerm)
	os.MkdirAll("/tmp/cache/ssd/", os.ModeDir|os.ModePerm)
	os.MkdirAll("/tmp/cache/mem/", os.ModeDir|os.ModePerm)

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

	go func() {
		time.Sleep(time.Minute)
		c.Dump()
	}()

	fmt.Println("add jpg")
	jpgkey := md5.Sum([]byte("http://www.test.com/123/456/1.jpg"))
	fmt.Println(c.Get(jpgkey, 0, -1)) // nil, nil
	jpg := []byte("this is 1.jpg")
	c.AddItem(jpgkey, time.Now().Unix()+3600, int64(len(jpg)), httpAuxData{
		fileType: crc32.ChecksumIEEE([]byte("jpg")),
		rawKey:   []byte("http://www.test.com/123/456/1.jpg")})
	c.AddSegment(jpgkey, 0, jpg)
	fmt.Println(c.Get(jpgkey, 0, -1))

	fmt.Println("add png")
	pngkey := md5.Sum([]byte("http://www.test.com/123/456/1.png"))
	fmt.Println(c.Get(pngkey, 0, -1)) // nil, nil
	png := []byte("this is 1.png")
	c.AddItem(pngkey, time.Now().Unix()+3600, int64(len(jpg)), httpAuxData{
		fileType: crc32.ChecksumIEEE([]byte("png")),
		rawKey:   []byte("http://www.test.com/123/456/1.png")})
	c.AddSegment(pngkey, 0, png)
	fmt.Println(c.Get(pngkey, 0, -1))

	fmt.Println("Del jpg")
	c.DelBatch(func(aux cache.Auxiliary) []cache.Hash {
		keys := make([]cache.Hash, 0)
		for k, v := range aux.(*httpAux).datas {
			if v.fileType == crc32.ChecksumIEEE([]byte("jpg")) {
				keys = append(keys, k)
			}
		}
		return keys
	})
	fmt.Println(c.Get(jpgkey, 0, -1))
	fmt.Println(c.Get(pngkey, 0, -1))

	fmt.Println("Del regex")
	r := regexp.MustCompile(`http://www.test.com/123/.*png`)
	c.DelBatch(func(aux cache.Auxiliary) []cache.Hash {
		keys := make([]cache.Hash, 0)
		for k, v := range aux.(*httpAux).datas {
			if r.Match(v.rawKey) {
				fmt.Println("match", string(v.rawKey))
				keys = append(keys, k)
			}
		}
		return keys
	})
	fmt.Println(c.Get(jpgkey, 0, -1))
	fmt.Println(c.Get(pngkey, 0, -1))
}
