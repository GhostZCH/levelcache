package levelcache

import (
	"encoding/gob"
	"os"
)

func success(args ...interface{}) {
	if args[len(args)-1] != nil {
		panic(args[len(args)-1])
	}
}

func safeDump(path string, data interface{}) {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_RDWR|os.O_CREATE, 0600)
	success(err)
	success(gob.NewEncoder(f).Encode(data))
	success(f.Close())
	success(os.Rename(tmp, path))
}
