package distributedCache

import (
	"distributedCache/consistentHash"
	"distributedCache/pb"
	"fmt"
	"google.golang.org/protobuf/proto"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

const defaultBasePath = "/_cache/"
const defaultReplicas = 50

// 实现服务端

// HTTPPool 承载节点间 HTTP 通信的核心数据结构
type HTTPPool struct {
	self        string                 // 保存自己的地址
	basePath    string                 // 通讯地址的前缀，默认是 /_cache/
	mu          sync.Mutex             // 保证节点选择时的并发安全
	peers       *consistentHash.Map    // 类型是一致性哈希算法的 Map，用来根据具体的 key 选择节点
	httpClients map[string]*httpClient // 映射远程节点地址和对应的 httpClient, 每一个远程节点对应一个 httpClient，因为 httpClient 与远程节点的地址 baseURL 有关
}

// Log 日志显示服务名
func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

// NewHTTPPool 初始化一个 HTTPPool
func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

// ServeHTTP 实现 http 方法
func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// 如果请求不是以 默认前缀开始的则报错
	if !strings.HasPrefix(req.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path: " + req.URL.Path)
	}
	// 显示请求方法和路径
	p.Log("%s %s", req.Method, req.URL.Path)
	// SplitN: s为待分割字符串，sep为分隔符，n为返回的字符串数
	// /<basepath>/<groupname>/<key> 得到的是 groupname 和 key，也就是parts
	parts := strings.SplitN(req.URL.Path[len(p.basePath):], "/", 2)
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
	}
	groupName := parts[0]
	key := parts[1]
	// 通过 groupName 得到 group 实例,也就是缓存的名字
	group := GetGroup(groupName)
	if group == nil {
		http.Error(w, "no such group: "+groupName, http.StatusNotFound)
		return
	}
	// 获取缓存数据
	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// 把结果以 proto 的格式写入到响应体中
	body, err := proto.Marshal(&pb.Response{Value: view.ByteSlice()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	// 将缓存值作为 httpResponse 的 body 返回
	w.Write(body)
}

// Set 实例化了一致性哈希算法，并且添加了传入的节点， 并为每一个节点创建了一个 HTTP 客户端 httpGetter
func (p *HTTPPool) Set(addrs ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// 实例化一个一致性哈希算法并采用默认的哈希函数
	p.peers = consistentHash.New(defaultReplicas, nil)
	// 添加节点，也就是真实的计算机节点
	p.peers.Add(addrs...)
	// 为每一个节点创建一个客户端并保存在 map 中
	p.httpClients = make(map[string]*httpClient, len(addrs))
	for _, addr := range addrs {
		// http://localhost:8001/_cache/
		p.httpClients[addr] = &httpClient{baseUrl: addr + p.basePath}
	}
}

// PickPeer 包装了一致性哈希算法的 Get() 方法，根据具体的 key，选择节点，返回节点对应的 HTTP 客户端
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// 根据 key 获取应该访问的节点地址
	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		// peer 是根据 key 查找到的计算机节点 URL
		p.Log("Pick peer from %s, key = %s", peer, key)
		// 返回对应于这个请求地址的客户端实例
		return p.httpClients[peer], true
	}
	return nil, false
}

// 检查 HTTPPool 是否实现了 PeerPicker 的全部接口
var _ PeerPicker = (*HTTPPool)(nil)

// 实现客户端

// httpGetter 客户端核心数据结构
type httpClient struct {
	baseUrl string // 表示将要访问的远程节点的地址
}

// Get 实现了 PickGetter 的 Get 方法获取返回值并转化为 []byte 类型
func (h *httpClient) Get(in *pb.Request, out *pb.Response) error {
	//log.Printf("http.go: Get -> h.baseUrl = %v, group = %v, key = %v\n", h.baseUrl, url.QueryEscape(group), url.QueryEscape(key))
	// 拼接要请求的 URL: 如 http://localhost:8001/_cache/ + groupName + key
	u := fmt.Sprintf("%v%v/%v", h.baseUrl, url.QueryEscape(in.Group), url.QueryEscape(in.Key))
	// 向服务端发起请求获取缓存值
	res, err := http.Get(u)
	// 请求失败，没有获取到对应的缓存
	if err != nil {
		return err
	}
	defer res.Body.Close()
	// 响应失败，返回响应错误信息
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned: %v", res.Status)
	}

	// 把响应体的内容转化为 bytes 类型
	bytes, err := ioutil.ReadAll(res.Body)
	// 读取响应体失败
	if err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}
	// 使用 proto.Unmarshal() 解码 HTTP 响应
	if err = proto.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("decoding response body: %v", err)
	}
	return nil
}

// 检查 httpClients 是否实现 PeerGetter 的全部的接口
var _ PeerGetter = (*httpClient)(nil)
