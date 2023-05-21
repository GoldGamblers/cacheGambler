package main

import (
	"distributedCache"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

var db = map[string]string{
	"Tom":  "666",
	"Jack": "777",
	"Sam":  "888",
}

func createGroup() *distributedCache.Group {
	return distributedCache.NewGroup("scores", 2<<10, distributedCache.GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("[SlowDB] search key", key)
			if v, ok := db[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))
}

// 用来启动缓存服务器：创建 HTTPPool，添加节点信息，注册到 distributedCache 中，启动 HTTP 服务（共3个端口，8001/8002/8003），用户不感知。
func startCacheServer(addr string, addrs []string, cacheGroup *distributedCache.Group) {
	// addr 是当前端口号对应的计算机节点的 URL, addrs 是所有计算机节点的 URL
	//log.Printf("main.go: startCacheServer -> addr = %s, addrs = %v\n", addr, addrs)
	peers := distributedCache.NewHTTPPool(addr)
	peers.Set(addrs...)
	// 注册所有的 计算机节点
	cacheGroup.RegisterPeers(peers)
	log.Println("distributedCache is running at", addr)
	log.Fatal(http.ListenAndServe(addr[7:], peers))
}

// 用来启动一个 API 服务（端口 9999），与用户进行交互，用户感知
func startAPIServer(apiAddr string, cacheGroup *distributedCache.Group) {
	http.Handle("/api", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			key := r.URL.Query().Get("key")
			//  根据 key 来查找
			view, err := cacheGroup.Get(key)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(view.ByteSlice())

		}))
	log.Println("API server is running at", apiAddr)
	log.Fatal(http.ListenAndServe(apiAddr[7:], nil))
}

// 需要命令行传入 port 和 api 2 个参数，用来在指定端口启动 HTTP 服务
func main() {
	var port int
	var api bool
	flag.IntVar(&port, "port", 8001, "distributedCache server port")
	flag.BoolVar(&api, "api", false, "Start a api server?")
	flag.Parse()

	apiAddr := "http://localhost:9999"
	addrMap := map[int]string{
		8001: "http://localhost:8001",
		8002: "http://localhost:8002",
		8003: "http://localhost:8003",
	}

	var addrs []string
	for _, v := range addrMap {
		addrs = append(addrs, v)
	}
	cache := createGroup()
	if api {
		go startAPIServer(apiAddr, cache)
	}
	time.Sleep(time.Second)
	startCacheServer(addrMap[port], []string(addrs), cache)
}
