package down

import (
	"testing"
)

// TestFormatFileSize 测试字节的单位转换
func TestFormatFileSize(t *testing.T) {
	testData := []struct {
		size       int64
		formatSize string
	}{
		{-1, "0.00 B"},
		{0, "0.00 B"},
		{1, "1.00 B"},
		{627, "627.00 B"},
		{1024, "1.00 KB"},
		{1025, "1.00 KB"},
		{2042, "1.99 KB"},
		{1048576, "1.00 MB"},
		{1073741824, "1.00 GB"},
		{1099511627776, "1.00 TB"},
		{1.1259e+15, "1.00 EB"},
	}

	for _, val := range testData {
		tmp := formatFileSize(val.size)
		if tmp != val.formatSize {
			t.Errorf("%v 测试失败, 输出为: %s 应该为: %v\n", val, tmp, val.formatSize)
		}
	}

}
