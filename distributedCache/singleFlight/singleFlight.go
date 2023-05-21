package singleFlight

import "sync"

// call 代表正在进行中，或已经结束的请求
type call struct {
	// 并发协程之间如不需要消息传递，非常适合 sync.WaitGroup
	// 本质是一个内部计数器，在执行goroutine行为之前执行 wg.Add(1)，给计数器+1，
	// 执行完之后，执行wg.Done()，表示这个goroutine执行完成，计数器内部-1，
	// wg.Wait()会阻塞代码的运行，等待所有的添加进WaitGroup的goroutine全部执行完毕(计数器减为0)，再退出程序
	wg  sync.WaitGroup // 使用 sync.WaitGroup 锁避免重入
	val interface{}    // 保存任意值
	err error
}

// SingleFlight 是 singleflight 的主数据结构，管理不同 key 的请求(call)
type SingleFlight struct {
	mu sync.Mutex // 保护 m 并发安全
	m  map[string]*call
}

// Do 针对相同的 key，无论 Do 被调用多少次，函数 fn 都只会被调用一次，等待 fn 调用结束了，返回返回值或错误
func (sf *SingleFlight) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	// g.mu 是保护 Group 的成员变量 m 不被并发读写而加上的锁
	sf.mu.Lock()
	// 还没有 key 和 call 的 map, 延迟初始化
	if sf.m == nil {
		sf.m = make(map[string]*call)
	}
	// 注意 sync.WaitGroup 和 sync.Mutex 的区别
	// 如果当前的 key 已经存在于 map 中，说明已经有相同的 key 的请求，此时等待请求结束，返回请求的结果，不必再次发起请求
	if c, ok := sf.m[key]; ok {
		sf.mu.Unlock()
		// 如果请求正在进行中，则等待
		c.wg.Wait()
		// 请求结束，返回结果
		return c.val, c.err
	}
	// 如果当前的 key 不存在 map 中，说明还没有相同的 key 的请求，需要发起
	c := new(call)
	// 发起请求前加锁
	c.wg.Add(1)
	// 添加到 g.m，表明 key 已经有对应的请求在处理
	sf.m[key] = c
	sf.mu.Unlock()
	// 调用 fn，发起请求
	c.val, c.err = fn()
	// 请求结束
	c.wg.Done()

	// 更新 g.m
	sf.mu.Lock()
	delete(sf.m, key)
	sf.mu.Unlock()
	// 返回结果
	return c.val, c.err
}
