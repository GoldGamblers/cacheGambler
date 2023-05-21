package consistentHash

import (
	"fmt"
	"hash/crc32"
	"sort"
	"strconv"
)

// 实现一致性哈希算法

//Hash 函数类型 Hash，采取依赖注入的方式，允许用于替换成自定义的 Hash 函数，也方便测试时替换
type Hash func(data []byte) uint32

// Map 一致性哈希算法的主数据结构
type Map struct {
	hash     Hash           // Hash 函数
	replicas int            // 虚拟节点倍数
	keys     []int          // 哈希环
	hashMap  map[int]string // 虚拟节点和真实节点的映射表，key 虚拟节点哈希值，值是真是节点名称
}

// New 构造函数
func New(replicas int, fn Hash) *Map {
	m := &Map{
		hash:     fn,
		replicas: replicas,
		hashMap:  make(map[int]string),
	}
	// 如果用户没有传入哈希函数则默认使用 crc32.ChecksumIEEE
	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

// Add 添加节点,也就是真实的机器
func (m *Map) Add(addrs ...string) {
	fmt.Printf("consistentHash.go: ADD() -> addrs = %s\n", addrs)
	for _, addr := range addrs {
		// 每一个真实节点创建 replicas 个虚拟节点
		for i := 0; i < m.replicas; i++ {
			// 计算虚拟节点的哈希值
			hash := int(m.hash([]byte(strconv.Itoa(i) + addr)))
			// 加入到环中
			m.keys = append(m.keys, hash)
			// 建立虚拟节点和真实节点的映射关系
			m.hashMap[hash] = addr
		}
	}
	// 对环重新排序
	sort.Ints(m.keys)
}

// Get 获取节点
func (m *Map) Get(key string) string {
	fmt.Printf("consistentHash.go: Get() -> key = %s\n", key)
	// 没有环
	if len(m.keys) == 0 {
		return ""
	}
	// 计算key的哈希值
	hash := int(m.hash([]byte(key)))
	// 顺时针找到第一个匹配的虚拟节点的下标
	index := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})
	// 通过虚拟节点找到真实节点, 取余是为了防止越界，因为是一个环
	return m.hashMap[m.keys[index%len(m.keys)]]
}
