package distributedCache

import (
	"distributedCache/lru"
	"sync"
)

//实现cache的并发控制，实例化 lru，封装 get 和 add 方法，并添加互斥锁 mu

// 实现并发特性
type cache struct {
	mu         sync.Mutex
	lru        *lru.CacheLRU // 采用 LRU 策略
	cacheBytes int64         // 最大的缓存空间
}

// add 封装 LRU 的 Add 方法
func (c *cache) add(key string, value ByteView) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// 延迟初始化(Lazy Initialization) : 意味着该对象的创建将会延迟至第一次使用该对象时
	// 主要用于提高性能，并减少程序内存要求
	if c.lru == nil {
		c.lru = lru.New(c.cacheBytes, nil)
	}
	c.lru.Add(key, value)
}

// find 封装 LRU 的 Find 方法
func (c *cache) find(key string) (value ByteView, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		return
	}

	if v, ok := c.lru.Find(key); ok {
		return v.(ByteView), ok
	}
	return
}
