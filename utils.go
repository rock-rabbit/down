package down

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"math/rand"
	"mime"
	"net/url"
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
			i++
			if i == 255 {
				break
			}
		}
		name = c.String()
	}

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
// size 随机码的位数
// kind 0=纯数字,1=小写字母,2=大写字母,3=数字、大小写字母
func randomString(size int, kind int) string {
	if size < 1 {
		return ""
	}
	if kind < 0 {
		kind = 0
	}
	ikind, kinds, rsbytes := kind, [][]int{{10, 48}, {26, 97}, {26, 65}}, make([]byte, size)
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
	return !os.IsNotExist(err)
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

// formatFileSize 字节的单位转换 保留两位小数
func formatFileSize(fileSize int64) (size string) {
	if fileSize < 0 {
		return "0.00 B"
	}
	if fileSize < 1024 {
		return fmt.Sprintf("%.2f B", float64(fileSize)/float64(1))
	} else if fileSize < (1024 * 1024) {
		return fmt.Sprintf("%.2f KB", float64(fileSize)/float64(1024))
	} else if fileSize < (1024 * 1024 * 1024) {
		return fmt.Sprintf("%.2f MB", float64(fileSize)/float64(1024*1024))
	} else if fileSize < (1024 * 1024 * 1024 * 1024) {
		return fmt.Sprintf("%.2f GB", float64(fileSize)/float64(1024*1024*1024))
	} else if fileSize < (1024 * 1024 * 1024 * 1024 * 1024) {
		return fmt.Sprintf("%.2f TB", float64(fileSize)/float64(1024*1024*1024*1024))
	} else {
		return fmt.Sprintf("%.2f EB", float64(fileSize)/float64(1024*1024*1024*1024*1024))
	}
}

// threadTask 多线程任务分割
func threadTaskSplit(size, threadSize int64) [][2]int64 {
	taskCountFloat64 := float64(size) / float64(threadSize)
	if math.Trunc(taskCountFloat64) != taskCountFloat64 {
		taskCountFloat64++
	}
	taskCount := int(taskCountFloat64)
	task := make([][2]int64, int(taskCount))
	for i := 0; i < taskCount; i++ {
		task[i][0] = int64(i) * threadSize

		task[i][1] = int64(i+1)*threadSize - 1
		if task[i][1] >= size {
			task[i][1] = size - 1
		}
	}
	return task
}

// getFileName 自动获取资源文件名称
// 名称获取的顺序：响应头 content-disposition 的 filename 字段、uri.Path 中的 \ 最后的字符、随机生成
// 文件后缀的获取顺序：文件魔数、响应头 content-type 匹配系统中的库
func getFileName(uri, contentDisposition, contentType string, headinfo []byte) string {
	// 尝试在响应中获取文件名称
	_, params, _ := mime.ParseMediaType(contentDisposition)
	if name, ok := params["filename"]; ok && name != "" {
		return name
	}
	// 尝试从 uri 中获取名称
	var (
		name, ext string
	)
	u, _ := url.Parse(uri)
	if u != nil {
		us := strings.Split(u.Path, "/")
		if len(us) > 1 {
			name = us[len(us)-1]
		}
	}
	// 尝试在文件魔数获取文件后缀
	fileType := getFileType(headinfo)
	if fileType != "" {
		ext = fmt.Sprintf(".%s", fileType)
	}
	if ext == "" {
		// 尝试从 content-type 中获取文件后缀
		extlist, _ := mime.ExtensionsByType(contentType)
		if len(extlist) != 0 {
			ext = extlist[0]
		}
	}
	if fname := filterFileName(name); name != "" && fname != "" {
		if strings.HasSuffix(fname, ext) {
			return fname
		}
		return fmt.Sprintf("%s%s", fname, ext)
	}
	// 名称获取失败时随机生成名称
	return fmt.Sprintf("file_%s%d%s", randomString(5, 1), time.Now().UnixNano(), ext)
}
