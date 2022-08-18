package down

import (
	"bytes"
	"io"
	"math/rand"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"
)

// filterFileName 根据操作系统规定过滤掉非法字符
func filterFileName(name string) string {
	switch runtime.GOOS {
	case "windows":
		return filterFileNameFormWindows(name)
	case "darwin":
		return filterFileNameFormDarwin(name)
	}
	return name
}
func filterFileNameFormWindows(name string) string {
	// 过滤头部的空格
	name = strings.TrimPrefix(name, regexGetOne(`^([[:blank:]]+)`, name))
	// 过滤非法字符
	for _, v := range []rune{'?', '\\', '/', '*', '"', '<', '>', '|', ':'} {
		name = strings.ReplaceAll(name, string(v), "")
	}
	// 截取前 255 个字
	if getStringLength(name) > 255 {
		i := 0
		c := bytes.NewBufferString("")
		for _, v := range name {
			c.WriteString(string(v))
			if i == 255 {
				break
			}
			i++
		}
		name = c.String()
	}

	return name
}

func filterFileNameFormDarwin(name string) string {
	// 过滤头部的 .
	name = strings.TrimPrefix(name, regexGetOne(`^([\.]+)`, name))
	// 过滤非法字符
	name = strings.ReplaceAll(name, ":", "")
	return name
}

// getStringLength 获取字符串的长度
func getStringLength(str string) int {
	return utf8.RuneCountInString(str)
}

// regexGetOne 获取匹配到的单个字符串
func regexGetOne(str, s string) string {
	re := regexp.MustCompile(str)
	submatch := re.FindStringSubmatch(s)
	if len(submatch) <= 1 {
		return ""
	}
	return submatch[1]
}

// randomString 随机数
func randomString(size int, kind int) string {
	ikind, kinds, rsbytes := kind, [][]int{[]int{10, 48}, []int{26, 97}, []int{26, 65}}, make([]byte, size)
	isAll := kind > 2 || kind < 0
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < size; i++ {
		if isAll { // random ikind
			ikind = rand.Intn(3)
		}
		scope, base := kinds[ikind][0], kinds[ikind][1]
		rsbytes[i] = uint8(base + rand.Intn(scope))
	}
	return string(rsbytes)
}

// fileExist 判断文件是否存在
func fileExist(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return true
}

// ioProxyReader 代理 io 读
type ioProxyReader struct {
	reader io.Reader
	send   func(n int)
}

// Read 读
func (r *ioProxyReader) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	r.send(n)
	return n, err
}

// Close the wrapped reader when it implements io.Closer
func (r *ioProxyReader) Close() (err error) {
	if closer, ok := r.reader.(io.Closer); ok {
		return closer.Close()
	}
	return
}
