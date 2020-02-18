package levelcache

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const (
	minBlockSize      int64 = 1024 * 1024
	maxBlockSize      int64 = 1024 * 1024 * 1024 * 10
	defaultBlockCount int64 = 1024
)

type store struct {
	dir       string
	cap       int64
	size      int64
	blockSize int64
	curOff    int64
	curBlock  int64
	lock      sync.RWMutex
	blocks    map[int64][]byte
}

func getBlockSize(capacity int64) int64 {
	// blockSize = min(max(minBlockSize, capacity / defaultBlockCount), maxBlockSize)
	size := int64(capacity / defaultBlockCount)
	if size < minBlockSize {
		size = minBlockSize
	}
	if size > maxBlockSize {
		size = maxBlockSize
	}
	return size

}

func newStore(dir string, cap int64) *store {
	blockSize := getBlockSize(cap)

	s := &store{
		dir:       dir,
		cap:       cap,
		size:      0,
		blockSize: blockSize,
		curOff:    blockSize,
		curBlock:  0,
		blocks:    make(map[int64][]byte)}

	blocks, err := filepath.Glob(fmt.Sprintf("%s/*.blk", dir))
	success(err)

	var b int64
	pattern := fmt.Sprintf("%s/%d-%%d.blk", dir, version)
	for _, path := range blocks {
		fmt.Sscanf(path, pattern, &b)
		data := s.mmap(path, 0)
		s.blocks[b] = data
		s.size += int64(len(data))
	}

	return s
}

func (s *store) get(sv *segValue) []byte {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if b, ok := s.blocks[sv.Block]; ok {
		return b[sv.Off : sv.Off+int64(sv.Size)]
	}
	return nil
}

func (s *store) add(data []byte) (block int64, off int64) {
	s.lock.Lock()
	defer s.lock.Unlock()

	size := int64(len(data))
	if size > s.blockSize {
		s.addBlock(size) // single block for big data
	} else if size+s.curOff > s.blockSize {
		s.addBlock(s.blockSize)
	}

	block = s.curBlock
	off = int64(s.curOff)

	copy(s.blocks[s.curBlock][s.curOff:s.curOff+size], data)
	s.curOff += size

	return block, off
}

func (s *store) clear() (blocks []int64) {
	s.lock.Lock()
	defer s.lock.Unlock()

	for len(s.blocks) >= 0 && s.size > s.cap {
		min, data := s.minBlock()
		path := s.getPath(min)

		s.size -= int64(len(data))
		delete(s.blocks, min)
		blocks = append(blocks, min)

		success(os.Remove(path))
		success(syscall.Munmap(data))
	}

	return blocks
}

func (s *store) close() {
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, data := range s.blocks {
		syscall.Munmap(data)
	}
}

func (s *store) minBlock() (min int64, data []byte) {
	min = 0
	for i, d := range s.blocks {
		if i < min || min == 0 {
			min, data = i, d
		}
	}
	return min, data
}

func (s *store) addBlock(size int64) {
	s.curBlock = time.Now().UnixNano()
	s.blocks[s.curBlock] = s.mmap(s.getPath(s.curBlock), int64(s.blockSize))
	s.curOff = 0
	s.size += size
}

func (s *store) getPath(block int64) string {
	return fmt.Sprintf("%s/%d-%016x.dat", s.dir, version, block)
}

func (s *store) mmap(path string, size int64) (data []byte) {
	if size == 0 {
		info, e1 := os.Stat(path)
		success(e1)
		size = info.Size()
	}

	f, e2 := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	success(e2)
	defer f.Close()
	success(f.Truncate(size))

	data, err := syscall.Mmap(int(f.Fd()), 0, int(size),
		syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	success(err)
	return data
}
