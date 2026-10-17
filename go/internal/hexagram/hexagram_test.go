package hexagram

import "testing"

func TestCodes(t *testing.T) {
	l := Lines{9, 8, 7, 6, 8, 7}
	if got := CodeOf(l); got != "101001" {
		t.Errorf("CodeOf = %s", got)
	}
	if got := ChangedCodeOf(l); got != "001101" {
		t.Errorf("ChangedCodeOf = %s", got)
	}
	if got := ChangingPositions(l); len(got) != 2 || got[0] != 1 || got[1] != 4 {
		t.Errorf("ChangingPositions = %v", got)
	}
	if got := ChangedCodeOf(Lines{7, 8, 7, 8, 7, 8}); got != "101010" {
		t.Errorf("无变爻时变卦应等于本卦，得 %s", got)
	}
	if ValidateLine(5) == nil || ValidateLine(10) == nil || ValidateLine(6) != nil {
		t.Error("ValidateLine 边界错误")
	}
}

func TestIndexRoundtrip(t *testing.T) {
	for i := 0; i < Dim; i++ {
		got, err := CodeToIndex(IndexToCode(i))
		if err != nil || got != i {
			t.Fatalf("roundtrip %d -> %d (%v)", i, got, err)
		}
	}
	if i, _ := CodeToIndex("100000"); i != 1 {
		t.Errorf("初爻应是最低位，得 %d", i)
	}
	for _, bad := range []string{"", "10101", "1010101", "10102a"} {
		if _, err := CodeToIndex(bad); err == nil {
			t.Errorf("%q 应报错", bad)
		}
	}
}

func TestDerived(t *testing.T) {
	cases := []struct{ got, want, what string }{
		{Cuo("100010"), "011101", "屯错=鼎"},
		{Zong("100010"), "010001", "屯综=蒙"},
		{Hu("100010"), "000001", "屯互=剥"},
		{Hu("111111"), "111111", "乾互=乾"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %s want %s", c.what, c.got, c.want)
		}
	}
}

func TestPermutationsAreInvolutiveBijections(t *testing.T) {
	seenC, seenZ := map[int]bool{}, map[int]bool{}
	for i := 0; i < Dim; i++ {
		c := IndexToCode(i)
		if Cuo(Cuo(c)) != c || Zong(Zong(c)) != c {
			t.Fatalf("%s 的错/综不是对合", c)
		}
		if PermCuo[PermCuo[i]] != i || PermZong[PermZong[i]] != i {
			t.Fatalf("置换在 %d 处不是对合", i)
		}
		seenC[PermCuo[i]], seenZ[PermZong[i]] = true, true
	}
	if len(seenC) != Dim || len(seenZ) != Dim {
		t.Error("置换不是双射，不能作为酉算子")
	}
}

func TestHuIsManyToOne(t *testing.T) {
	images := map[string]bool{}
	for i := 0; i < Dim; i++ {
		images[Hu(IndexToCode(i))] = true
	}
	if len(images) != 16 {
		t.Errorf("互卦像集大小 = %d，应为 16", len(images))
	}
}
