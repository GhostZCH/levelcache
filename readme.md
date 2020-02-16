

`level cache`是一个多级的缓存库，方便将nfs,hdd,ssd,mem等分级使用实现高效划算的对较大对象缓存，例如http的文件缓存。

##　功能

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

    type Hash [md5.Size]byte

    type Segment struct {
        Index int32 //Segment index
        Block int64
        Off   int64
        Size  int64
    }

    type Item struct {
        Key      Hash
        Expire   int64
        RawKey   []byte
        Tags     []uint32 //for delete or remember something like etag, header-long, file-type, owner
        SegSize  uint32
        Segments map[int32]*Segment
    }

    type DevConf struct {
        Path     string
        Capacity int
    }

    type Config struct {
        ActionParallel int
    }

    type Matcher func(*Item) bool

    func NewCache(conf Config, devices []DevConf) *Cache

    func (c *Cache) Dump()

    func (c *Cache) GetItem(key Hash) (level int, item *Item)

    func (c *Cache) Add(item *Item)

    func (c *Cache) AddSegment(key Hash, index int, seg Segment)

    func (c *Cache) Del(key Hash)

    func (c *Cache) DelBatch(m Matcher)

## TODO

+ 完善readme
+ 数据统计
+ 改进热点对象加入高级别缓存逻辑
+ rawkey可以用sqlite等数据库存储，从内存mete分离
+ 增量备份