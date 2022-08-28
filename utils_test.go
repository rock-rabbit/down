package down

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// TestFilterFileNameWindows windows 规定过滤掉非法字符
func TestFilterFileNameWindows(t *testing.T) {
	names := [][2]string{
		{"这在济南是合法的?", "这在济南是合法的"},
		{"大明湖\\红叶谷\\千佛山，这些都很好玩", "大明湖红叶谷千佛山，这些都很好玩"},
		{"最好玩的地方是融创乐园/融创乐园", "最好玩的地方是融创乐园融创乐园"},
		{"哔哔***哔哔", "哔哔哔哔"},
		{"宝贝\"我爱你\"", "宝贝我爱你"},
		{"这里输入<文件名>", "这里输入文件名"},
		{"济南:山东的省会", "济南山东的省会"},
		{strings.Repeat("1", 256), strings.Repeat("1", 255)},
	}

	for _, v := range names {
		tmp := filterFileNameFormWindows(v[0])
		if tmp != v[1] {
			t.Errorf("过滤掉非法字符失败, 输出 %s, 应输出 %s", tmp, v[1])
		}
	}
}

// TestGetStringLength 获取字符串的长度
func TestGetStringLength(t *testing.T) {
	testData := []struct {
		content string
		lenght  int
	}{
		{"", 0},
		{"rockrabbit", 10},
		{"我是一名网络民工", 8},
	}
	for _, v := range testData {
		tmp := getStringLength(v.content)
		if tmp != v.lenght {
			t.Errorf("获取字符串的长度失败, 输出 %d, 应输出 %d", tmp, v.lenght)
		}
	}
}

// TestRegexGetOne 获取匹配到的单个字符串
func TestRegexGetOne(t *testing.T) {
	testData := []struct {
		regex   string
		content string
		out     string
	}{
		{``, "", ""},
		{`数量：(\d+)`, "---数量：827", "827"},
		{`数量：\d+`, "---数量：827", ""},
		{`数量：\d+---(\d+)`, "---数量：827---928---", "928"},
	}
	for _, v := range testData {
		tmp := regexGetOne(v.regex, v.content)
		if tmp != v.out {
			t.Errorf("匹配单个字符串失败, 输出 %s, 应输出 %s", tmp, v.out)
		}
	}
}

// TestRandomString 测试随机数生成
func TestRandomString(t *testing.T) {
	testData := []struct {
		size       int
		kind       int
		correctLen int
	}{
		{-1, 0, 0},
		{1, -1, 1},
		{1, 3, 1},
		{0, 0, 0},
		{1, 0, 1},
		{5, 0, 5},
		{10, 0, 10},
		{107, 0, 107},
		{1000, 0, 1000},
		{10000, 0, 10000},
	}
	for _, v := range testData {
		tmp := randomString(v.size, v.kind)
		if len(tmp) != v.correctLen {
			t.Errorf("随机数生成失败, 获得 %d 个字符, 应该获得 %d 个字符", len(tmp), v.correctLen)
		}
	}
}

// TestFileExist 测试文件是否存在
func TestFileExist(t *testing.T) {
	testFile := []struct {
		filepath string
		exist    bool
	}{
		{"./utils.go", true},
		{"./utils_nil_nil.go", false},
	}

	for _, v := range testFile {
		tmp := fileExist(v.filepath)
		if tmp != v.exist {
			t.Errorf("测试文件是否存在失败, 获得 %v, 应该为 %v", tmp, v.exist)
		}
	}
}

// TestIoProxyReader 测试代理 io
func TestIoProxyReader(t *testing.T) {
	t.Run("closer", func(t *testing.T) {
		testStr := "rockrabbit"

		testReader := io.NopCloser(bytes.NewBuffer([]byte(testStr)))
		lenght := 0
		ioproxy := &ioProxyReader{reader: testReader}
		ioproxy.send = func(n int) {
			lenght += n
		}
		ioproxy.Close()
		_, err := io.ReadAll(ioproxy)
		if err != nil {
			t.Error(err)
			return
		}
		if lenght != 10 {
			t.Errorf("代理IO失败, 获得 %d 字节, 应该获得 %d 字节", lenght, 10)
		}
	})

	t.Run("reader", func(t *testing.T) {
		testStr := "rockrabbit"

		testReader := bytes.NewBuffer([]byte(testStr))
		lenght := 0
		ioproxy := &ioProxyReader{reader: testReader}
		ioproxy.send = func(n int) {
			lenght += n
		}
		ioproxy.Close()
		_, err := io.ReadAll(ioproxy)
		if err != nil {
			t.Error(err)
			return
		}
		if lenght != 10 {
			t.Errorf("代理IO失败, 获得 %d 字节, 应该获得 %d 字节", lenght, 10)
		}
	})

}

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

func TestThreadTaskSplit(t *testing.T) {
	testData := []struct {
		size, threadSize int64
		out              [][2]int64
	}{
		{0, 0, [][2]int64{}},
		{2048, 1024, [][2]int64{{0, 1023}, {1024, 2047}}},
		{2049, 1024, [][2]int64{{0, 1023}, {1024, 2047}, {2048, 2048}}},
		{2047, 1024, [][2]int64{{0, 1023}, {1024, 2046}}},
		{0, 1024, [][2]int64{}},
	}

	for _, v := range testData {
		tmp := threadTaskSplit(v.size, v.threadSize)
		if len(tmp) != len(v.out) {
			t.Fatalf("size:%d threadSize:%d 任务分割失败, 长度不一致，输出 %v, 应输出 %v", v.size, v.threadSize, tmp, v.out)
		}
		for idx, vv := range tmp {
			if vv != v.out[idx] {
				t.Fatalf("size:%d threadSize:%d 任务分割失败, 输出 %v, 应输出 %v", v.size, v.threadSize, tmp, v.out)
			}
		}
	}
}

func TestThreadTaskSplitBreakpoint(t *testing.T) {
	testData := []struct {
		start, size, threadSize int64
		out                     [][2]int64
	}{
		{0, 0, 0, [][2]int64{}},
		{1000, 2048, 1024, [][2]int64{{1000, 2023}, {2024, 2047}}},
		{1000, 2049, 1024, [][2]int64{{1000, 2023}, {2024, 2048}}},
		{1000, 2047, 1024, [][2]int64{{1000, 2023}, {2024, 2046}}},
		{1000, 0, 1024, [][2]int64{}},
	}

	for _, v := range testData {
		tmp := threadTaskSplitBreakpoint(v.start, v.size, v.threadSize)
		if len(tmp) != len(v.out) {
			t.Fatalf("size:%d threadSize:%d 任务分割失败, 长度不一致，输出 %v, 应输出 %v", v.size, v.threadSize, tmp, v.out)
		}
		for idx, vv := range tmp {
			if vv != v.out[idx] {
				t.Fatalf("size:%d threadSize:%d 任务分割失败, 输出 %v, 应输出 %v", v.size, v.threadSize, tmp, v.out)
			}
		}
	}
}

func TestGetFileName(t *testing.T) {
	testData := []struct {
		uri                string
		contentDisposition string
		contentType        string
		headinfo           []byte
		out                string
	}{
		{"", "", "", []byte{}, "file"},
		{"test.com", "", "", []byte{}, "file"},
		{"test.com", "attachment;filename=2022-12.xlsx", "", []byte{}, "2022-12.xlsx"},
		{"test.com/file", "", "application/postscript", []byte{}, "file.ai"},
		{"test.com/file", "", "", []byte{80, 75, 3, 4, 20, 0, 0, 0, 8, 0}, "file.zip"},
		{"test.com/2022-12.xlsx?s=521", "", "", []byte{}, "2022-12.xlsx"},
	}
	for _, v := range testData {
		tmp := getFileName(v.uri, v.contentDisposition, v.contentType, v.headinfo)
		if tmp != v.out && !strings.HasPrefix(tmp, v.out) {
			t.Errorf("获取文件名称失败, 输出 %s, 应输出 %s", tmp, v.out)
		}
	}
}
