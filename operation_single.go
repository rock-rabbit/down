package down

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync/atomic"
)

// single 单线程，非断点续传
func (operat *operation) single() {
	defer operat.ctxCance()

	f, err := os.OpenFile(operat.outputPath, os.O_CREATE|os.O_RDWR, operat.meta.Perm)
	if err != nil {
		operat.done <- fmt.Errorf("open file: %s", err)
		return
	}
	defer f.Close()

	if err := f.Truncate(operat.size); err != nil {
		operat.done <- err
		return
	}

	res, err := operat.defaultDo(nil)
	if err != nil {
		operat.done <- err
		return
	}
	defer res.Body.Close()

	// 每秒给 Hook 发送信息
	go operat.sendStat(nil)
	// 磁盘缓冲区
	bufWriter := bufio.NewWriterSize(f, operat.down.DiskCache)
	// 使用代理 io 写入文件
	_, err = io.Copy(bufWriter, &ioProxyReader{reader: res.Body, send: func(n int) {
		select {
		case <-operat.ctx.Done():
			res.Body.Close()
		default:
			atomic.AddInt64(operat.stat.CompletedLength, int64(n))
		}
	}})
	// 缓冲数据写入到磁盘
	bufWriter.Flush()
	// 判断是否有错误
	select {
	case <-operat.ctx.Done():
		operat.done <- operat.ctx.Err()
	default:
		if err != nil {
			operat.done <- err
			return
		}
		operat.finishHook()
	}

	operat.done <- nil
}

// singleBreakpoint 单线程，断点续传
func (operat *operation) singleBreakpoint() {
	operat.finish(operat.err)
}
