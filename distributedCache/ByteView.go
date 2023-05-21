package distributedCache

// 实现缓存值的抽象与封装

// ByteView 抽象的数据结构表示缓存值
type ByteView struct {
	b []byte // 存储真实的缓存值，选择 byte 类型是为了能够支持任意的数据类型的存储
}

// Len 在 lru.Cache 的实现中，要求被缓存对象必须实现 Value 接口，即 Len() int 方法，返回其所占的内存大小
func (v ByteView) Len() int {
	return len(v.b)
}

// ByteSlice 返回一个 ByteView 的切片， []byte 类型
// b 是只读的，使用 ByteSlice() 方法返回一个拷贝，防止缓存值被外部程序修改
func (v ByteView) ByteSlice() []byte {
	return cloneBytes(v.b)
}

// String 返回一个 ByteView 的字符串类型的数据
func (v ByteView) String() string {
	return string(v.b)
}

// cloneBytes
func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
