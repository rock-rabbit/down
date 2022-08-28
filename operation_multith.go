package down

import (
	"bufio"
	"io"
	"sync/atomic"
)

// multith 多线程下载
func (operat *operation) multith() {
	if err := operat.operatFile.file.Truncate(operat.size); err != nil {
		operat.err = err
		operat.finish(err)
		return
	}
	task := threadTaskSplit(operat.size, int64(operat.down.ThreadSize))
	// 任务执行
	groupPool := NewWaitGroupPool(operat.down.ThreadCount)
	// 自动保存控制文件
	go operat.operatCF.autoSave(operat.down.AutoSaveTnterval)
	// 每秒给 Hook 发送信息
	go operat.sendStat(groupPool)
	// 执行多线程任务
	go operat.startMultith(groupPool, task)
	// 阻塞等待所有线程完成后返回结果
	for err := range groupPool.AllDone() {
		if err != nil {
			operat.err = err
		} else {
			operat.finish(operat.err)
			return
		}
	}
}

// startMultith 执行多线程
func (operat *operation) startMultith(groupPool *WaitGroupPool, task [][2]int64) {
	for idx, fileRange := range task {
		groupPool.Add()
		// 中途关闭
		if operat.contextIsDone() {
			groupPool.Done()
			break
		}
		operat.operatCF.addTB(0, fileRange[0], fileRange[1])
		go operat.multithSingle(idx, groupPool, fileRange[0], fileRange[1])
	}
	// 非阻塞等待所有任务完成
	groupPool.Syne()
}

// multithSingle 多线程下载中单个线程的下载逻辑
func (operat *operation) multithSingle(id int, groupPool *WaitGroupPool, rangeStart, rangeEnd int64) {
	defer groupPool.Done()
	res, err := operat.rangeDo(rangeStart, rangeEnd)
	if err != nil {
		groupPool.Error(err)
		return
	}
	defer res.Body.Close()
	// 硬盘缓冲区大小
	size := int(rangeEnd - rangeStart + 1)
	bufSize := operat.operatFile.bufsize
	if bufSize > size {
		bufSize = size
	}
	// 新建硬盘缓冲区写入
	buf := bufio.NewWriterSize(operat.operatFile.makeFileAt(id, rangeStart), bufSize)
	_, err = io.Copy(buf, &ioProxyReader{reader: res.Body, send: func(n int) {
		atomic.AddInt64(operat.stat.CompletedLength, int64(n))
	}})
	if err != nil {
		groupPool.Error(err)
		return
	}
	// 存盘
	if err := buf.Flush(); err != nil {
		groupPool.Error(err)
	}
}

// multithBreakpoint 多线程，断点续传
func (operat *operation) multithBreakpoint() {
	operat.finish(operat.err)
}
