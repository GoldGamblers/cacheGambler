package lru

import (
	"container/list"
)

// CacheLRU LRU 缓存，并发访问是不安全的。
type CacheLRU struct {
	maxBytes  int64                         // 允许使用的最大内存
	nbytes    int64                         // 当前已使用的内存
	ll        *list.List                    // 使用 Go 语言标准库实现的双向链表list.List
	cacheMap  map[string]*list.Element      // 维护一个map， 键是字符串，值是双向链表中对应节点的指针
	OnEvicted func(key string, value Value) // 某条记录被移除时的回调函数
}

// node 键值对 node 是双向链表节点的数据类型
type node struct {
	key   string
	value Value
}

// Value 为了通用性，值是实现了 Value 接口的任意类型，该接口只包含了一个方法 Len() int，用于返回值所占用的内存大小
type Value interface {
	Len() int
}

// New 实例化一个 CacheLRU
func New(maxBytes int64, OnEvicted func(key string, value Value)) *CacheLRU {
	return &CacheLRU{
		maxBytes:  maxBytes,
		OnEvicted: OnEvicted,
		ll:        list.New(),
		cacheMap:  make(map[string]*list.Element),
	}
}

// Find 实现查找功能,第一步是从字典中找到对应的双向链表的节点，第二步，将该节点移动到队尾
func (c *CacheLRU) Find(key string) (value Value, ok bool) {
	if elem, ok := c.cacheMap[key]; ok {
		// 双向链表作为队列，队首队尾是相对的，在这里约定 Back 为队尾
		c.ll.MoveToBack(elem)
		// 返回节点值
		kv := elem.Value.(*node)
		return kv.value, true
	}
	return
}

// Remove 移除最近最少访问的节点,也就是队首的元素
func (c *CacheLRU) Remove() {
	// 双向链表作为队列，队首队尾是相对的，在这里约定 Back 为队尾, Front为队首
	elem := c.ll.Front()
	if elem != nil {
		//fmt.Printf("elem = %v\n", elem.Value)
		// 从双向列表中移除
		c.ll.Remove(elem)
		kv := elem.Value.(*node)
		// 删除这个节点
		delete(c.cacheMap, kv.key)
		// 更新当前已使用内存
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
		// 如果有回调函数则调用
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

// Add 新增一个节点
func (c *CacheLRU) Add(key string, value Value) {
	// 如果这个节点已经存在则修改并移动到队尾
	if elem, ok := c.cacheMap[key]; ok {
		c.ll.MoveToBack(elem)
		kv := elem.Value.(*node)
		// 计算已使用内存情况
		c.nbytes += int64(len(kv.key)) + int64(kv.value.Len())
		kv.value = value
	} else {
		// 节点不存在则创建并添加到队尾
		elem := c.ll.PushBack(&node{key: key, value: value})
		c.cacheMap[key] = elem
		c.nbytes += int64(len(key)) + int64(value.Len())
	}
	// 如果内存使用超过了最大值则调用 remove 来淘汰
	for c.maxBytes != 0 && c.maxBytes < c.nbytes {
		c.Remove()
	}
}

// GetRecord 用于获取添加了多少条数据,测试方便一些
func (c *CacheLRU) GetRecord() int {
	return c.ll.Len()
}
