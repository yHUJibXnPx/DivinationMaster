package rng

import "testing"

func TestSplitMix64KnownConstant(t *testing.T) {
	// 业界公认的参考值：种子 0 的第一个输出
	if _, out := splitmix64(0); out != 0xE220A8397B1DCDAF {
		t.Errorf("splitmix64(0) = %#x", out)
	}
}

func TestDeterministicAndRange(t *testing.T) {
	a, b := New(42), New(42)
	for i := 0; i < 100; i++ {
		if a.Uint64() != b.Uint64() {
			t.Fatal("同种子应得到同序列")
		}
	}
	r := New(1)
	for i := 0; i < 10000; i++ {
		if f := r.Float64(); f < 0 || f >= 1 {
			t.Fatalf("Float64 越界: %v", f)
		}
		if c := r.Coin(); c != 0 && c != 1 {
			t.Fatalf("Coin 越界: %d", c)
		}
	}
}
