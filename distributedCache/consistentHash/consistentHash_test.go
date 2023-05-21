package consistentHash

import (
	"strconv"
	"testing"
)

// 如果要进行测试，那么我们需要明确地知道每一个传入的 key 的哈希值，那使用默认的 crc32.ChecksumIEEE 算法显然达不到目的
// 所以在这里使用了自定义的 Hash 算法。自定义的 Hash 算法只处理数字，传入字符串表示的数字，返回对应的数字即可
func TestHashing(t *testing.T) {
	// 构造函数，使用自定义的哈希函数
	hash := New(3, func(key []byte) uint32 {
		i, _ := strconv.Atoi(string(key))
		return uint32(i)
	})

	// 初始有 2,4,6 三个节点,对应的虚拟头结点是 02,12,22 ... 以此类推
	hash.Add("6", "4", "2")
	// key的用例中 分别选择的虚拟节点是02,12,24,02，真实节点是 2,2,4,2,
	testCases := map[string]string{
		"2":  "2",
		"11": "2",
		"23": "4",
		"27": "2",
	}
	for k, v := range testCases {
		if hash.Get(k) != v {
			t.Errorf("Asking for %s, should have yielded %s", k, v)
		}
	}

	// 添加节点 8, 对应的虚拟头结点是 08,18,28
	hash.Add("8")
	// key的用例 27 对应的虚拟节点应该从 02 变为 28
	testCases["27"] = "8"
	for k, v := range testCases {
		if hash.Get(k) != v {
			t.Errorf("Asking for %s, should have yielded %s", k, v)
		}
	}
}
