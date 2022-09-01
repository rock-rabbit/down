package down_test

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/rock-rabbit/down"
)

func TestDown(t *testing.T) {

	outpath := "./tmp"
	meta := []string{"http://127.0.0.1:25427/down.bin", outpath, "down0.bin"}
	metaMerging := [][2]string{
		{"http://127.0.0.1:25427/down.bin", "down1.bin"},
		{"http://127.0.0.1:25427/down.bin", "down2.bin"},
	}

	remove := func() {
		os.RemoveAll("./tmp")
	}

	down.AddHook(down.DefaultBarHook)

	t.Run("关闭断点续传", func(t *testing.T) {
		defer remove()
		defer testserver(t, time.Second*1)()

		down.SetThreadCount(2)
		down.SetContinue(false)

		path, err := down.Run(meta...)
		if err != nil {
			tmppath := filepath.Join(path, ".down")
			_, err := os.Stat(tmppath)
			if !os.IsNotExist(err) {
				log.Panic("关闭断点续传后，临时文件还存在")
			}
			return
		}
		log.Panic("文件下载成功了，应该断开连接下载失败才对")
	})

	t.Run("单线程-正常下载", func(t *testing.T) {
		defer remove()
		defer testserver(t, 0)()

		down.SetThreadCount(1)

		path, err := down.Run(meta...)
		if err != nil {
			log.Panic(err)
		}
		fmt.Println("文件下载完成：" + path)
	})

	t.Run("单线程-下载中途失败", func(t *testing.T) {
		defer remove()
		defer testserver(t, time.Second*1)()

		down.SetThreadCount(1)

		path, err := down.Run(meta...)
		if err != nil {
			fmt.Println(err)
			return
		}
		log.Panic("文件下载完成：" + path)
	})

	t.Run("单线程-正常合并下载", func(t *testing.T) {
		defer remove()
		defer testserver(t, 0)()

		down.SetThreadCount(1)

		path, err := down.RunMerging(metaMerging, outpath)
		if err != nil {
			log.Panic(err)
		}
		fmt.Println("文件下载完成：", path)
	})

	t.Run("多线程-正常下载", func(t *testing.T) {
		defer remove()
		defer testserver(t, 0)()

		down.SetThreadCount(3)

		path, err := down.Run(meta...)
		if err != nil {
			log.Panic(err)
		}
		fmt.Println("文件下载完成：" + path)
	})

	t.Run("多线程-下载中途失败", func(t *testing.T) {
		defer remove()
		defer testserver(t, time.Second*1)()

		down.SetThreadCount(3)

		path, err := down.Run(meta...)
		if err != nil {
			fmt.Println(err)
			return
		}
		log.Panic("文件下载完成：" + path)
	})

	t.Run("多线程-正常合并下载", func(t *testing.T) {
		defer remove()
		defer testserver(t, 0)()

		down.SetThreadCount(3)

		path, err := down.RunMerging(metaMerging, outpath)
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
		io.Copy(w, &down.IoProxyReader{Reader: bufio.NewReaderSize(buf, 1024), Send: func(n int) {
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
