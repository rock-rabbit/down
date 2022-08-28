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
	buf := bufio.NewWriterSize(operat.operatFile.makeFileAt(0, 0, 0), bufSize)
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
	// 自动保存控制文件
	go operat.operatCF.autoSave(operat.down.AutoSaveTnterval)
	// 每秒给 Hook 发送信息
	go operat.sendStat(nil)
	// 已分配的数据块
	var err error
	for id, block := range operat.operatCF.getCF().threadblock {
		if block.completed == (block.end-block.start)+1 {
			continue
		}
		err = operat.singleBreakpointBlock(id, block.start+block.completed, block.end, block.completed)
		if err != nil {
			operat.err = err
			operat.finish(err)
			return
		}
	}
	// 未分配的任务块
	blockallsize := operat.operatCF.getCF().threadblock[len(operat.operatCF.getCF().threadblock)-1].end + 1
	if operat.size > blockallsize {
		operat.operatCF.addTB(0, blockallsize, operat.size-1)
		id := len(operat.operatCF.getCF().threadblock) - 1
		err = operat.singleBreakpointBlock(id, blockallsize, operat.size-1, 0)
		if err != nil {
			operat.err = err
			operat.finish(err)
			return
		}
	}
	operat.finish(nil)
}

// singleBreakpointBlock 断点续传单数据块
func (operat *operation) singleBreakpointBlock(id int, start, end, completed int64) error {
	res, err := operat.rangeDo(start, end)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	// 硬盘缓冲区大小
	size := int(end - start + 1)
	bufSize := operat.operatFile.bufsize
	if bufSize > size {
		bufSize = size
	}
	// 新建硬盘缓冲区写入
	buf := bufio.NewWriterSize(operat.operatFile.makeFileAt(id, start, completed), bufSize)
	_, err = io.Copy(buf, &ioProxyReader{reader: res.Body, send: func(n int) {
		atomic.AddInt64(operat.stat.CompletedLength, int64(n))
	}})
	if err != nil {
		return err
	}
	// 存盘
	if err := buf.Flush(); err != nil {
		return err
	}
	return nil
}
