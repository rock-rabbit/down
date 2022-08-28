package down

import (
	"bufio"
	"io"
	"sync/atomic"
)

// single 单线程，非断点续传
func (operat *operation) single() {
	if err := operat.operatFile.file.Truncate(operat.size); err != nil {
		operat.err = err
		operat.finish(err)
		return
	}
	// 自动保存控制文件
	go operat.operatCF.autoSave(operat.down.AutoSaveTnterval)
	// 每秒给 Hook 发送信息
	go operat.sendStat(nil)
	// 执行下载任务
	operat.operatCF.addTB(0, 0, operat.size-1)
	res, err := operat.defaultDo(nil)
	if err != nil {
		operat.err = err
		operat.finish(err)
		return
	}
	defer res.Body.Close()
	// 硬盘缓冲区大小
	bufSize := operat.operatFile.bufsize
	if bufSize > int(operat.size) {
		bufSize = int(operat.size)
	}
	// 新建硬盘缓冲区写入
	buf := bufio.NewWriterSize(operat.operatFile.makeFileAt(0, 0), bufSize)
	_, err = io.Copy(buf, &ioProxyReader{reader: res.Body, send: func(n int) {
		atomic.AddInt64(operat.stat.CompletedLength, int64(n))
	}})
	if err != nil {
		operat.err = err
		operat.finish(err)
		return
	}
	// 存盘
	if err := buf.Flush(); err != nil {
		operat.err = err
		operat.finish(err)
		return
	}
	operat.finish(nil)
}

// singleBreakpoint 单线程，断点续传
func (operat *operation) singleBreakpoint() {
	operat.finish(operat.err)
}
