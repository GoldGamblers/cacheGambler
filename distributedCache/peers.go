package distributedCache

import "distributedCache/pb"

// 抽象两个接口

// PeerPicker 用于根据传入的 key 选择对应的 PeerGetter
type PeerPicker interface {
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// PeerGetter 用于从对应的 group 缓存中查找缓存值，也就是 HTTP 客户端，之前已经实现了提供缓存的服务端
type PeerGetter interface {
	Get(in *pb.Request, out *pb.Response) error
}
