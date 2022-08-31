package down_test

import (
	"bufio"
	"bytes"
	"context"
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

	meta := down.NewMeta("http://127.0.0.1:25427/down.bin", "./tmp", "")
	meta2 := down.NewMeta("http://127.0.0.1:25427/down.bin", "./tmp", "down2.bin")

	down.Default.AddHook(down.DefaultBarHook)

	t.Run("单线程-正常下载", func(t *testing.T) {
		defer testserver(t, 0)()

		down.Default.ThreadCount = 1

		path, err := down.Default.Run(meta)
		if err != nil {
			log.Panic(err)
		}
		fmt.Println("文件下载完成：" + path)
	})

	t.Run("单线程-下载中途失败", func(t *testing.T) {
		defer testserver(t, time.Second*1)()

		down.Default.ThreadCount = 1

		path, err := down.Default.Run(meta)
		if err != nil {
			fmt.Println(err)
			return
		}
		log.Panic("文件下载完成：" + path)
	})

	t.Run("单线程-正常合并下载", func(t *testing.T) {
		defer testserver(t, 0)()

		down.Default.ThreadCount = 1

		path, err := down.Default.RunMerging([]*down.Meta{meta, meta2})
		if err != nil {
			log.Panic(err)
		}
		fmt.Println("文件下载完成：", path)
	})

	t.Run("多线程-正常下载", func(t *testing.T) {
		defer testserver(t, 0)()

		down.Default.ThreadCount = 3

		path, err := down.Default.Run(meta)
		if err != nil {
			log.Panic(err)
		}
		fmt.Println("文件下载完成：" + path)
	})

	t.Run("多线程-下载中途失败", func(t *testing.T) {
		defer testserver(t, time.Second*1)()

		down.Default.ThreadCount = 3

		path, err := down.Default.Run(meta)
		if err != nil {
			fmt.Println(err)
			return
		}
		log.Panic("文件下载完成：" + path)
	})

	t.Run("多线程-正常合并下载", func(t *testing.T) {
		defer testserver(t, 0)()

		down.Default.ThreadCount = 3

		path, err := down.Default.RunMerging([]*down.Meta{meta, meta2})
		if err != nil {
			log.Panic(err)
		}
		fmt.Println("文件下载完成：", path)
	})

}

func testserver(t *testing.T, timeout time.Duration) func() {
	ctx, cancel := context.WithCancel(context.Background())
	if timeout != 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	}
	done := make(chan bool)
	go rundownserve(t, ctx, done)
	return func() {
		cancel()
		<-done
	}
}

func rundownserve(t *testing.T, ctx context.Context, done chan bool) {

	size := 1024 << 17

	handmux := http.NewServeMux()

	handmux.HandleFunc("/down.bin", func(w http.ResponseWriter, r *http.Request) {
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

	serve := &http.Server{
		Addr:         ":25427",
		Handler:      handmux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go serve.ListenAndServe()

	<-ctx.Done()

	serve.Close()

	done <- true

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
