package down_test

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/rock-rabbit/down"
)

func TestDown(t *testing.T) {
	go TestRunDownServe(t)
	// 创建一个基本下载信息
	meta := down.NewMeta("http://127.0.0.1:25427/down.bin", "./tmp", "")
	// 添加一个请求头
	meta.Header.Set("referer", "https://im.qq.com/")
	// down.Default 为默认配置的下载器, 你可以查看 Down 结构体配置自己的下载器
	down.Default.AddHook(down.DefaultBarHook)
	// down.Default.ThreadSize = 1024 << 10
	down.Default.ThreadCount = 1
	// 执行下载, 你也可以使用 RunContext 传递一个 Context
	path, err := down.Default.Run(meta)
	if err != nil {
		log.Panic(err)
	}
	fmt.Println("文件下载完成：" + path)
}

func TestRunDownServe(t *testing.T) {

	size := 1024 << 20

	http.HandleFunc("/down.bin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Add("Accept-Ranges", "bytes")
			w.Header().Add("Content-Length", fmt.Sprint(size))
			return
		}
		headRange := r.Header.Get("range")
		var buf *bytes.Buffer
		if headRange != "" {
			rgxrange := regexp.MustCompile(`bytes=(\d+)-(\d+)`).FindStringSubmatch(headRange)
			if len(rgxrange) != 3 {
				w.WriteHeader(500)
				return
			}
			start, err := strconv.ParseInt(rgxrange[1], 10, 0)
			if err != nil {
				w.WriteHeader(500)
				return
			}
			end, err := strconv.ParseInt(rgxrange[2], 10, 0)
			if err != nil {
				w.WriteHeader(500)
				return
			}
			w.Header().Add("content-range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
			w.Header().Add("content-length", fmt.Sprint(end-start+1))
			w.WriteHeader(206)
			buf = bytes.NewBuffer(make([]byte, end-start+1))
		} else {
			w.WriteHeader(200)
			w.Header().Add("Content-Length", fmt.Sprint(size))
			buf = bytes.NewBuffer(make([]byte, size))
		}
		io.Copy(w, &ioProxyReader{reader: bufio.NewReaderSize(buf, 1024), send: func(n int) {
			time.Sleep(time.Duration(time.Millisecond) * 1)
		}})
	})
	http.ListenAndServe(":25427", nil)
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
