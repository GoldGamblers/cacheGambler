package lru

import (
	"reflect"
	"testing"
)

type String string

func (s String) Len() int {
	return len(s)
}

func TestCacheLRU_Find(t *testing.T) {
	lru := New(int64(0), nil)
	lru.Add("key1", String("1234"))
	if v, ok := lru.Find("key1"); !ok || string(v.(String)) != "1234" {
		t.Fatalf("cache hit key1=1234 failed")
	}
	if _, ok := lru.Find("key2"); ok {
		t.Fatalf("cache miss key2 failed")
	}
}

func TestCacheLRU_Remove(t *testing.T) {
	k1, k2, k3 := "key1", "key2", "k3"
	v1, v2, v3 := "value1", "value2", "v3"
	cap := len(k1 + k2 + v1 + v2)
	//fmt.Printf("cap = %v\n", cap)
	lru := New(int64(cap), nil)
	lru.Add(k1, String(v1))
	lru.Add(k2, String(v2))
	lru.Add(k3, String(v3))

	if _, ok := lru.Find("key1"); ok || lru.GetRecord() != 2 {
		t.Fatalf("Removeoldest key1 failed")
	}
}

func TestOnEvicted(t *testing.T) {
	keys := make([]string, 0)
	callback := func(key string, value Value) {
		// 把移除的节点的值保存起来放在后面进行对比
		keys = append(keys, key)
	}
	lru := New(int64(10), callback)
	// 第一个键值对就将缓存沾满了。
	lru.Add("key1", String("123456"))
	// 弹出 key1
	lru.Add("k2", String("k2"))
	lru.Add("k3", String("k3"))
	// 弹出 k2
	lru.Add("k4", String("k4"))
	// 检查是否弹出 key1 和 k2
	expect := []string{"key1", "k2"}

	if !reflect.DeepEqual(expect, keys) {
		t.Fatalf("Call OnEvicted failed, expect keys equals to %s", expect)
	}
}
