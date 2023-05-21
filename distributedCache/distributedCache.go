package distributedCache

import (
	"distributedCache/pb"
	"distributedCache/singleFlight"
	"fmt"
	"log"
	"sync"
)

// 核心的数据结构，负责与用户的交互，并且控制缓存值存储和获取的流程

// 以下是接口型函数的实现：
// 定义了一个接口 Getter，只包含一个方法 Get(key string) ([]byte, error)
// 紧接着定义了一个函数类型 GetterFunc，GetterFunc 参数和返回值与 Getter 中 Get 方法是一致的。
// 而且 GetterFunc 还定义了 Get 方式，并在 Get 方法中调用自己，这样就实现了接口 Getter。
// 所以 GetterFunc 是一个实现了接口的函数类型，简称为接口型函数。
// 接口型函数只能应用于接口内部只定义了一个方法的情况，例如接口 Getter 内部有且只有一个方法 Get

// Getter 接口隔离数据源，由用户决定数据源的种类，这里提供 Get回调方法，当缓存不存在时获取数据源
type Getter interface {
	Get(key string) ([]byte, error)
}

// GetterFunc 定义一个函数类型并实现 Getter 接口的 Get 方法
type GetterFunc func(key string) ([]byte, error)

// Get 回调方法实现
func (f GetterFunc) Get(key string) ([]byte, error) {
	// 匿名函数的执行
	return f(key)
}

// 定义和初始化一些常使用的变量
var (
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

// Group 定义：一个 group 是一个缓存命名空间和相关的数据加载分布
type Group struct {
	name      string                     // 每个 Group 拥有一个唯一的名称 name
	getter    Getter                     // 缓存未命中时获取源数据的回调(callback)
	mainCache cache                      // 采用 LRU 实现的单机并发安全缓存
	peers     PeerPicker                 // 支持选择节点并获取对应节点的缓存数据
	loader    *singleFlight.SingleFlight // 使用 singleFlight, 保证相同的 key 只会发起一次请求
}

// NewGroup 实例化
func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
		loader:    &singleFlight.SingleFlight{},
	}
	groups[name] = g
	return g
}

// GetGroup 用来获取特定名称的 Group
func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

// Get 实现核心的 Get 方法，从缓存中通过 key 得到 value
func (g *Group) Get(key string) (ByteView, error) {
	// 如果 key 是空的
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}
	// 如果查找到了,返回缓存
	if v, ok := g.mainCache.find(key); ok {
		log.Println("[Cache] hit")
		return v, nil
	}
	// 没查找到，调用load方法
	return g.load(key)
}

// load load 调用 getLocally（分布式场景下会调用 getFromPeer 从其他节点获取)
func (g *Group) load(key string) (value ByteView, err error) {
	// 使用 g.loader.Do 包裹请求保证相同的 key 只请求一次
	signalFetch, err := g.loader.Do(key, func() (interface{}, error) {
		// 之前不能保证相同的 key 只 fetch 一次
		if g.peers != nil {
			// 使用 PickPeer() 方法选择节点，如果是非本机节点，则进入以下流程，调用 getFromPeer() 从远程获取
			if peer, ok := g.peers.PickPeer(key); ok {
				// 要从远程节点获取对应的 key 的缓存值， peer 是通过 key 查询到的远程节点的 URL
				if value, err := g.getFromPeer(peer, key); err == nil {
					//log.Printf("[Cache] request key is from [%s]\n", peer)
					return value, nil
				}
				// 若是本机节点或失败，则回退到 getLocally()
				log.Println("[Cache] Failed to get from peer", err)
			}
		}
		return g.getLocally(key)
	})
	if err == nil {
		return signalFetch.(ByteView), nil
	}
	return
}

// getLocally 调用用户回调函数 g.getter.Get() 获取源数据
func (g *Group) getLocally(key string) (ByteView, error) {
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}
	value := ByteView{b: cloneBytes(bytes)}
	// 通过 populateCache 方法将源数据添加到缓存 mainCache 中
	g.populateCache(key, value)
	return value, nil
}

// populateCache 将源数据添加到缓存 mainCache 中
func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}

// RegisterPeers 实现了 PeerPicker 接口的 HTTPPool 注入到 Group 中
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

// getFromPeer 使用实现了 PeerGetter 接口的 httpGetter 从访问远程节点，获取缓存值
func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	// 使用 protoc 通信
	req := &pb.Request{
		// name 是缓存的名字，key 是这个缓存中这个 key 的值
		Group: g.name,
		Key:   key,
	}
	res := &pb.Response{}
	err := peer.Get(req, res)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: res.Value}, err
}
