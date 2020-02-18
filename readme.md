

`level cache`是一个多级的缓存库，方便将nfs,hdd,ssd,mem等分级使用实现高效划算的对较大对象缓存，例如http的文件缓存。为方便大文件的存储，该库以分片（segment）为基本的存储单元。元数据和设备管理采用分桶机制，每次访问只需锁定一个桶，具有较高的效率。

## 设计

### 对象和分片 (Item & Segment)

### 设备分级 (Devices & Levels)

### 桶的设计(Useage of Buckets)

### 元数据结构（Meta）

### 附加数据（Auxiliary）

## 特点

+ 支持多级缓存，level越高，缓存速度越快，体积越小，自动热点移动到高级别的缓存中
+ 方便配置，可以挂载为linux目录的设备都可以作为缓存，配置参数相同，例如内存可以使用`/dev/shm/`
+ 支持分片存储，根据自定义的分片大小，只存储部分内容
+ 在按照存储key删除单个对象的基础上，支持并行批量删除功能,按照item的属性自定义是否删除
+ 使用若干个大文件缓存大量对象，防止系统产生大量小文件，FIFO过期方式，基本man
+ 不同设备采用相对的对象管理代码，代码简洁，易于二次开发
+ 采用分桶逻辑，每次增删操作只需要锁定一个桶(1/256≈4‰)的数据，使用读写锁，并且只锁定读写元数据，不锁定返送数据
+ 块文件在初始化时通过mmap加载，不需要每次索引文件系统，文件不复制到内存
+ ...



## 接口

    // 存储key
    type Hash [md5.Size]byte

    // 设备初始化配置
    type DevConf struct {
        Name     string
        Dir      string
        Capacity int
    }

    // 缓存库公共配置
    type Config struct {
        MetaDir        string
        ActionParallel int
        AuxFactory     AuxFactory
    }

    // 缓存对象，用于操作缓存
    type Cache struct

    // 调用者附加数据接口，由调用者根据业务情况设计数据结构实现相关的方法，不需要调用者加锁
    type Auxiliary interface {
        Add(key Hash, auxItem interface{})
        Get(key Hash) interface{}
        Del(key Hash)
        Load(path string)
        Dump(path string)
    }

    // 产生附加数据的工厂函数，用作新建缓存的参数
    type AuxFactory func(idx int) Auxiliary

    // 批量删除的回调函数，用于批量删除时在用户自定义数据中查找需要删除的数据，每个桶执行一次，不需要调用者加锁
    type Matcher func(aux Auxiliary) []Hash

    // 初始化一个新的缓存对象
    func NewCache(conf Config, devices []DevConf) *Cache 

    // 关闭，保存文件，关闭句柄
    func (c *Cache) Close()

    // 保存元数据，建议周期性的调用，防止程序意外退出造成较大损失
    func (c *Cache) Dump()

    // 获得缓存对象某一部分的数据，end = -1 时表示获取到数据文件末尾,每次get后相应的数据分片可能会被调整到速度更快的缓存设备中
    // dataList为数据分片的列表
    // missSegments表示缺失的数据分片，每个元素[2]int的内容是对应分片的start与end
    func (c *Cache) Get(key Hash, start int, end int) (dataList [][]byte, missSegments [][2]int)

    // 增加一个缓存对象，只包含基础信息，不包含缓存数据和分片
    func (c *Cache) AddItem(key Hash, expire, size int64, auxData interface{})

    // 
    func (c *Cache) AddSegment(key Hash, start int, data []byte)

    func (c *Cache) Del(k Hash)

    func (c *Cache) DelBatch(m Matcher)

## 示例

详见`example/main.go`

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
        jpg := []byte("this is 1.jpg")
        c.AddItem(jpgkey, time.Now().Unix()+3600, int64(len(jpg)), httpAuxData{
            fileType: crc32.ChecksumIEEE([]byte("jpg")),
            rawKey:   []byte("http://www.test.com/123/456/1.jpg")})
        c.AddSegment(jpgkey, 0, jpg)
        fmt.Println(c.Get(jpgkey, 0, -1))

        fmt.Println("add png")
        pngkey := md5.Sum([]byte("http://www.test.com/123/456/1.png"))
        png := []byte("this is 1.png")
        c.AddItem(pngkey, time.Now().Unix()+3600, int64(len(jpg)), httpAuxData{
            fileType: crc32.ChecksumIEEE([]byte("png")),
            rawKey:   []byte("http://www.test.com/123/456/1.png")})
        c.AddSegment(pngkey, 0, png)
        fmt.Println(c.Get(jpgkey, 0, -1))

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
    }

## TODO

+ 完善readme
+ 数据统计
+ 改进热点对象加入高级别缓存逻辑
+ rawkey可以用sqlite等数据库存储，从内存mete分离
+ 增量备份