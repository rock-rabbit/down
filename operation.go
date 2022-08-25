package down

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// operation 下载前的配置拷贝结构, 防止多线程使用时的配置变化
type operation struct {
	down *Down
	meta *Meta

	// hooks 通过 Down 的 PerHook 生成的 Hook
	hooks []Hook

	// client 通过 Down 配置生成 *http.Client
	client *http.Client

	// size 文件大小
	size int64

	// multithread 是否使用多线程下载
	multithread bool

	// filename 从 URI 和 头信息 中获得的文件名称, 未指定名称时使用
	filename string

	// outputPath 最终文件的位置
	outputPath string

	// controlfilePath 控制文件位置
	controlfilePath string
	// cf 读取到的控制文件
	cf *controlfile

	// ctx 上下文
	ctx      context.Context
	ctxCance context.CancelFunc

	// done 下载完成
	done chan error

	// stat 下载进行时的进度记录
	stat *stating
}

// Stat 下载中发送给 Hook 的数据
type Stat struct {
	Meta *Meta
	Down *Down
	// TotalLength 文件大小
	TotalLength int64
	// CompletedLength 已下载的文件大小
	CompletedLength int64
	// DownloadSpeed 每秒下载字节数
	DownloadSpeed int64
	// OutputPath 最终文件的位置
	OutputPath string
	// Connections 与资源服务器的连接数
	Connections int
}

// stating 下载进行时的数据
type stating struct {
	CompletedLength *int64
}

// init 初始化，down 配置的应用
func (operat *operation) init() error {
	operat.client = &http.Client{
		Transport: &http.Transport{
			// 应用来自环境变量的代理
			Proxy: operat.down.Proxy,
			// 要求服务器返回非压缩的内容，前提是没有发送 accept-encoding 来接管 transport 的自动处理
			DisableCompression: true,
			// 等待响应头的超时时间
			ResponseHeaderTimeout: operat.down.ConnectTimeout,
			// TLS 握手超时时间
			TLSHandshakeTimeout: 10 * time.Second,
			// 接受服务器提供的任何证书
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		// 超时时间
		Timeout: 0,
	}

	var err error

	// 读取文件基础信息
	err = operat.baseInfo()
	if err != nil {
		return err
	}

	// 文件存储路径
	outputName := operat.meta.OutputName
	if outputName == "" {
		outputName = operat.filename
	}
	operat.outputPath, err = filepath.Abs(filepath.Join(operat.meta.OutputDir, outputName))
	if err != nil {
		return fmt.Errorf("filepath.Abs: %s", err)
	}
	operat.controlfilePath = fmt.Sprintf("%s.%s", operat.outputPath, operat.down.TempFileExt)

	// 文件是否存在, 这里之后支持断点续传后需要改逻辑
	outputPathExist := fileExist(operat.outputPath)
	// 清理文件
	clearFile := func() error {
		// 强制覆盖文件，清理文件
		err = os.Remove(operat.outputPath)
		if err != nil {
			return fmt.Errorf("remove file: %s", err)
		}
		err = os.Remove(operat.controlfilePath)
		if err != nil {
			return fmt.Errorf("remove file: %s", err)
		}
		return nil
	}

	if outputPathExist && operat.down.Continue && fileExist(operat.controlfilePath) {
		// 可以使用断点下载 并且 存在控制文件
		data, err := os.ReadFile(operat.controlfilePath)
		if err != nil {
			return err
		}
		cf := readControlfile(data)
		// 控制文件解析，解析失败允许删除则删除
		if (cf == nil || cf.totalLength != uint64(operat.size)) && outputPathExist {
			err = clearFile()
			if err != nil {
				return err
			}
		} else if cf == nil {
			return fmt.Errorf("已存在文件 %s，若要强制替换文件请将 down.AllowOverwrite 设为 true", operat.outputPath)
		}
		// 解析成功
		operat.cf = cf
		atomic.SwapInt64(operat.stat.CompletedLength, int64(operat.cf.completedLength))

	} else if outputPathExist && operat.down.AllowOverwrite {
		// 强制覆盖文件，清理文件
		err = clearFile()
		if err != nil {
			return err
		}
	} else if outputPathExist {
		return fmt.Errorf("已存在文件 %s，若要强制替换文件请将 down.AllowOverwrite 设为 true", operat.outputPath)
	}

	// 目录不存在时创建目录
	if operat.down.CreateDir && !fileExist(operat.meta.OutputDir) {
		os.MkdirAll(operat.meta.OutputDir, os.ModePerm)
	}

	// 生成 Hook
	operat.hooks = make([]Hook, len(operat.down.PerHooks))
	stat := &Stat{Down: operat.down, Meta: operat.meta, TotalLength: operat.size, OutputPath: operat.outputPath}
	for idx, perhook := range operat.down.PerHooks {
		operat.hooks[idx], err = perhook.Make(stat)
		if err != nil {
			return fmt.Errorf("Make Hook: %s", err)
		}
	}

	// 创建超时 context
	operat.ctx, operat.ctxCance = context.WithTimeout(operat.ctx, operat.down.Timeout)
	return nil
}

// baseInfo 获取资源基础信息，多线程支持的判断
func (operat *operation) baseInfo() error {
	res, err := operat.rangeDo(0, 9)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	contentType := res.Header.Get("content-type")
	contentDisposition := res.Header.Get("content-disposition")
	contentRange := res.Header.Get("content-range")
	contentLength := res.Header.Get("content-length")
	acceptRanges := res.Header.Get("accept-ranges")
	headinfo := []byte{}

	// 获取文件总大小
	rangeList := strings.Split(contentRange, "/")
	if len(rangeList) > 1 {
		operat.size, _ = strconv.ParseInt(rangeList[1], 10, 64)
	}

	// 是否可以使用多线程
	if acceptRanges != "" || strings.Contains(contentRange, "bytes") || contentLength == "10" {
		headinfo, _ = io.ReadAll(res.Body)
		operat.multithread = true
	} else {
		// 不支持多线程重新获取文件总大小
		if operat.size == 0 {
			operat.size, _ = strconv.ParseInt(contentLength, 10, 64)
		}
	}

	// 自动获取文件名称
	operat.filename = getFileName(operat.meta.URI, contentDisposition, contentType, headinfo)

	return nil
}

// start 开始执行下载
func (operat *operation) start() error {
	// 初始化操作
	if err := operat.init(); err != nil {
		return err
	}

	// 单线程下载逻辑
	if !operat.multithread || operat.down.ThreadCount <= 1 {
		go operat.singleThread()
	}

	// 多线程下载逻辑
	go operat.multithreading()

	return nil
}

// wait 等待下载完成
func (operat *operation) wait() error {
	return <-operat.done
}

// happenError 当出现错误时的处理方式
func (operat *operation) happenError(done <-chan int, f *os.File, cf *controlfile) {
	// 等待线程处理完成
	<-done
	// 保存控制文件
	operat.saveControlfile(f, cf)
}

// autoSaveControlfile 自动保存控制文件
func (operat *operation) autoSaveControlfile(f *os.File, cf *controlfile) {
	for {
		select {
		case <-time.After(operat.down.AutoSaveTnterval):
			operat.saveControlfile(f, cf)
		case <-operat.ctx.Done():
			return
		}
	}
}

// saveControlfile 保存控制文件
func (operat *operation) saveControlfile(f *os.File, cf *controlfile) {
	cf.completedLength = uint64(atomic.LoadInt64(operat.stat.CompletedLength))
	f.Seek(0, 0)
	io.Copy(f, cf.Encoding())
}

// contextIsDone 判断 context 是否关闭
func (operat *operation) contextIsDone() bool {
	select {
	case <-operat.ctx.Done():
		return true
	default:
		return false
	}
}

// multithreading 多线程下载
func (operat *operation) multithreading() {
	defer operat.ctxCance()

	f, err := os.OpenFile(operat.outputPath, os.O_CREATE|os.O_RDWR, operat.meta.Perm)
	if err != nil {
		operat.done <- fmt.Errorf("open file: %s", err)
		return
	}
	defer f.Close()
	// 非断点下载，设置文件大小
	if operat.cf == nil {
		if err := f.Truncate(operat.size); err != nil {
			operat.done <- err
			return
		}
	}
	// 拆分任务
	task := threadTaskSplit(operat.size, int64(operat.down.ThreadSize))

	// 非断点下载，创建控制文件
	var cf *controlfile
	if operat.cf == nil {
		cf = newControlfile(len(task))
		cf.threadSize = uint32(operat.down.ThreadSize)
		cf.totalLength = uint64(operat.size)
	} else {
		cf = operat.cf
	}
	cfIns, err := os.OpenFile(operat.controlfilePath, os.O_CREATE|os.O_RDWR, operat.meta.Perm)
	if err != nil {
		operat.done <- fmt.Errorf("open file: %s", err)
		return
	}
	defer cfIns.Close()

	// 任务执行
	groupPool := NewWaitGroupPool(operat.down.ThreadCount)
	done := make(chan int)
	cherr := make(chan error)

	// 自动保存控制文件
	go operat.autoSaveControlfile(cfIns, cf)
	// 每秒给 Hook 发送信息
	go operat.sendStat(groupPool)

	var downFunc func()
	if operat.cf == nil {
		downFunc = func() {
			for idx, fileRange := range task {
				groupPool.Add()
				// 被通知关闭了
				if operat.contextIsDone() {
					groupPool.Done()
					break
				}
				go operat.threadTask(groupPool, cherr, f, cf.threadblock[idx], fileRange[0], fileRange[1], 0)
			}
			groupPool.Wait()
			done <- 1
		}
	} else {
		downFunc = func() {
			for idx, tb := range operat.cf.threadblock {
				// 当前线程是否已完成
				completedLength := int64(tb.completedLength)
				if completedLength == (task[idx][1]-task[idx][0])+1 {
					continue
				}
				groupPool.Add()
				// 被通知关闭了
				if operat.contextIsDone() {
					groupPool.Done()
					break
				}
				go operat.threadTask(groupPool, cherr, f, cf.threadblock[idx], task[idx][0], task[idx][1], completedLength)
			}
			groupPool.Wait()
			done <- 1
		}
	}
	go downFunc()

	// 等待下载完成或失败
	select {
	case <-operat.ctx.Done():
		operat.happenError(done, cfIns, cf)
		operat.done <- errors.New("context 关闭")
		return
	case err = <-cherr:
		operat.happenError(done, cfIns, cf)
		operat.done <- err
		return
	case <-done:
		break
	}
	// 发送成功 Hook
	operat.finishHook()
	// 删除控制文件
	cfIns.Close()
	os.Remove(operat.controlfilePath)
	// 发送成功
	operat.done <- nil
}

// threadTask 多线程下载中单个线程的下载逻辑
func (operat *operation) threadTask(groupPool *WaitGroupPool, cherr chan error, f *os.File, tb *threadblock, rangeStart, rangeEnd, completed int64) {
	defer groupPool.Done()

	rangeStart = rangeStart + completed

	res, err := operat.rangeDo(rangeStart, rangeEnd)
	if err != nil {
		cherr <- err
		return
	}
	defer res.Body.Close()

	bufSize := (rangeEnd - rangeStart) + 1

	buf := bytes.NewBuffer(make([]byte, 0, bufSize))

	// 使用代理 io 写入文件
	_, err = io.Copy(buf, &ioProxyReader{reader: res.Body, send: func(n int) {
		atomic.AddInt64(operat.stat.CompletedLength, int64(n))
	}})
	select {
	case <-operat.ctx.Done():
		// 如果被取消，将缓冲区的数据写入到文件
		data := buf.Bytes()
		tb.completedLength = uint32(len(data) + int(completed))
		f.WriteAt(data, rangeStart)
		return
	default:
		if err != nil {
			cherr <- fmt.Errorf("io.Copy: %s", err)
			return
		}
	}
	// 写入到文件
	data := buf.Bytes()
	tb.completedLength = uint32(len(data))
	n, err := f.WriteAt(data, rangeStart)
	if err != nil {
		cherr <- err
		return
	}
	if int64(n) != bufSize {
		cherr <- fmt.Errorf("down error: bytes=%d-%d 写入数据为 %d 字节，与预计 %d 字节不符", rangeStart, rangeEnd, n, bufSize)
		return
	}
}

// singleThread 一个线程或者不支持多线程时的下载逻辑
func (operat *operation) singleThread() {
	defer operat.ctxCance()

	f, err := os.OpenFile(operat.outputPath, os.O_CREATE|os.O_RDWR, operat.meta.Perm)
	if err != nil {
		operat.done <- fmt.Errorf("open file: %s", err)
		return
	}
	defer f.Close()

	if operat.cf == nil {
		if err := f.Truncate(operat.size); err != nil {
			operat.done <- err
			return
		}
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
		operat.done <- fmt.Errorf("context 关闭")
	default:
		if err != nil {
			operat.done <- err
			return
		}
		operat.finishHook()
	}

	operat.done <- nil
}

// copyHooks 拷贝 Hook ，防止使用 Hook 中途发生变化
func (operat *operation) copyHooks() []Hook {
	var tmpHooks []Hook
	operat.down.mux.Lock()
	tmpHooks = make([]Hook, len(operat.hooks))
	copy(tmpHooks, operat.hooks)
	operat.down.mux.Unlock()
	return tmpHooks
}

// finishHook 下载完成时通知 Hook
func (operat *operation) finishHook() error {
	tmpHooks := operat.copyHooks()

	err := Hooks(tmpHooks).Finish(&Stat{
		Meta:        operat.meta,
		Down:        operat.down,
		TotalLength: operat.size,
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "down error: finish hook failure: %v\n", err)
	}
	return nil
}

// sendHook 下载途中给 Hook 发送下载信息如 下载速度、已下载大小、下载连接数等...
func (operat *operation) sendHook(stat *Stat) error {
	tmpHooks := operat.copyHooks()

	err := Hooks(tmpHooks).Send(stat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "down error: send hook failure: %v\n", err)
	}
	return nil
}

// sendStat 下载资源途中对数据的处理和发送 Hook
func (operat *operation) sendStat(groupPool *WaitGroupPool) {
	oldCompletedLength := atomic.LoadInt64(operat.stat.CompletedLength)
	for {
		select {
		case <-time.After(time.Second):
			connections := 1
			if groupPool != nil {
				connections = groupPool.Count()
			}
			completedLength := atomic.LoadInt64(operat.stat.CompletedLength)
			downloadSpeed := completedLength - oldCompletedLength
			oldCompletedLength = completedLength
			stat := &Stat{
				Meta:            operat.meta,
				Down:            operat.down,
				TotalLength:     operat.size,
				CompletedLength: completedLength,
				DownloadSpeed:   downloadSpeed,
				Connections:     connections,
				OutputPath:      operat.outputPath,
			}
			operat.sendHook(stat)
		case <-operat.ctx.Done():
			return
		}
	}
}
