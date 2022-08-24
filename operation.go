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
	// ctx 上下文
	ctx context.Context

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

func (operat *operation) start() error {
	// 初始化操作
	operat.init()
	// 读取文件基础信息
	operat.baseInfo()
	// 文件存储路径
	outputName := operat.meta.OutputName
	if outputName == "" {
		outputName = operat.filename
	}
	var err error
	operat.outputPath, err = filepath.Abs(filepath.Join(operat.meta.OutputDir, outputName))
	if err != nil {
		return fmt.Errorf("filepath.Abs: %s", err)
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
	// 文件是否存在, 这里之后支持断点续传后需要改逻辑
	if fileExist(operat.outputPath) {
		if !operat.down.AllowOverwrite {
			return fmt.Errorf("已存在文件 %s，若要强制替换文件请将 down.AllowOverwrite 设为 true", operat.outputPath)
		}
		// 需要强制覆盖, 删除掉原文件
		err = os.Remove(operat.outputPath)
		if err != nil {
			return fmt.Errorf("remove file: %s", err)
		}
	}
	// 目录不存在时创建目录
	if operat.down.CreateDir && !fileExist(operat.meta.OutputDir) {
		os.MkdirAll(operat.meta.OutputDir, os.ModePerm)
	}
	// 单线程下载逻辑
	if !operat.multithread || operat.down.ThreadCount <= 1 {
		if err := operat.singleThread(); err != nil {
			return err
		}
		return nil
	}
	// 多线程下载逻辑
	if err := operat.multithreading(); err != nil {
		return err
	}
	return nil
}

// happenError 当出现错误时的处理方式
func (operat *operation) happenError(done <-chan int, f *os.File, cf *controlfile) {
	// 等待线程处理完成
	<-done
	// 保存控制文件
	operat.saveControlfile(f, cf)
}

// autoSaveControlfile 自动保存控制文件
func (operat *operation) autoSaveControlfile(ctx context.Context, f *os.File, cf *controlfile) {
Loop:
	for {
		select {
		case <-ctx.Done():
			break Loop
		default:
			operat.saveControlfile(f, cf)
		}
		time.Sleep(operat.down.AutoSaveTnterval)
	}
}

// saveControlfile 保存控制文件
func (operat *operation) saveControlfile(f *os.File, cf *controlfile) {
	cf.completedLength = uint64(atomic.LoadInt64(operat.stat.CompletedLength))
	f.Seek(0, 0)
	io.Copy(f, cf.Encoding())
}

// multithreading 多线程下载
func (operat *operation) multithreading() error {
	f, err := os.OpenFile(operat.outputPath, os.O_CREATE|os.O_RDWR, operat.meta.Perm)
	if err != nil {
		return fmt.Errorf("open file: %s", err)
	}
	defer f.Close()

	if err := f.Truncate(operat.size); err != nil {
		return err
	}
	// 拆分任务
	task := threadTaskSplit(operat.size, int64(operat.down.ThreadSize))
	// 任务执行
	groupPool := NewWaitGroupPool(operat.down.ThreadCount)
	done := make(chan int)
	cherr := make(chan error)
	// 创建控制文件
	cf := newControlfile(len(task))
	cf.threadSize = uint32(operat.down.ThreadSize)
	cf.totalLength = uint64(operat.size)
	controlfilePath := fmt.Sprintf("%s.%s", operat.outputPath, operat.down.TempFileExt)
	controlfile, err := os.OpenFile(controlfilePath, os.O_CREATE|os.O_RDWR, operat.meta.Perm)
	if err != nil {
		return fmt.Errorf("open file: %s", err)
	}
	defer controlfile.Close()
	// 超时上下文，控制超时时间
	timeoutCtx, timeoutCancel := context.WithTimeout(operat.ctx, operat.down.Timeout)
	defer timeoutCancel()
	// 自动保存控制文件
	go operat.autoSaveControlfile(timeoutCtx, controlfile, cf)
	// 每秒给 Hook 发送信息
	go operat.sendStat(timeoutCtx, groupPool)
	go func() {
	Loop:
		for idx, fileRange := range task {
			groupPool.Add()
			select {
			case <-timeoutCtx.Done():
				// 被取消掉了
				groupPool.Done()
				break Loop
			default:
			}
			go operat.threadTask(timeoutCtx, groupPool, cherr, f, cf.threadblock[idx], fileRange[0], fileRange[1])
		}
		groupPool.Wait()
		done <- 1
	}()
	// 等待下载完成或失败
	select {
	case <-operat.ctx.Done():
		operat.happenError(done, controlfile, cf)
		return errors.New("context 关闭")
	case <-timeoutCtx.Done():
		operat.happenError(done, controlfile, cf)
		return errors.New("超时")
	case err = <-cherr:
		operat.happenError(done, controlfile, cf)
		return err
	case <-done:
		break
	}
	// 发送成功 Hook
	operat.finishHook()
	// 删除控制文件
	controlfile.Close()
	os.Remove(controlfilePath)
	return nil
}

// threadTask 多线程下载中单个线程的下载逻辑
func (operat *operation) threadTask(ctx context.Context, groupPool *WaitGroupPool, cherr chan error, f *os.File, tb *threadblock, rangeStart, rangeEnd int64) {
	defer groupPool.Done()
	req, err := operat.request(http.MethodGet, operat.meta.URI, operat.meta.Body)
	if err != nil {
		cherr <- fmt.Errorf("request: %s", err)
		return
	}
	req.Header.Set("range", fmt.Sprintf("bytes=%d-%d", rangeStart, rangeEnd))
	res, err := operat.do(req)
	if err != nil {
		cherr <- fmt.Errorf("request Do: %s", err)
		return
	}
	defer res.Body.Close()
	bufSize := (rangeEnd - rangeStart) + 1

	buf := bytes.NewBuffer(make([]byte, 0, bufSize))
	// 使用代理 io 写入文件
	_, err = io.Copy(buf, &ioProxyReader{reader: res.Body, send: func(n int) {
		select {
		case <-ctx.Done():
			res.Body.Close()
		default:
			atomic.AddInt64(operat.stat.CompletedLength, int64(n))
		}
	}})
	select {
	case <-ctx.Done():
		// 如果被取消，将缓冲区的数据写入到文件
		data := buf.Bytes()
		tb.completedLength = uint32(len(data))
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
func (operat *operation) singleThread() error {
	f, err := os.OpenFile(operat.outputPath, os.O_CREATE|os.O_RDWR, operat.meta.Perm)
	if err != nil {
		return fmt.Errorf("open file: %s", err)
	}
	defer f.Close()

	if err := f.Truncate(operat.size); err != nil {
		return err
	}

	req, err := operat.request(http.MethodGet, operat.meta.URI, nil)
	if err != nil {
		return fmt.Errorf("request: %s", err)
	}
	res, err := operat.do(req)
	if err != nil {
		return fmt.Errorf("request Do: %s", err)
	}
	defer res.Body.Close()
	// 超时上下文，控制超时时间
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), operat.down.Timeout)
	defer timeoutCancel()
	// 每秒给 Hook 发送信息
	go operat.sendStat(timeoutCtx, nil)
	// 磁盘缓冲区
	bufWriter := bufio.NewWriterSize(f, operat.down.DiskCache)
	// 使用代理 io 写入文件
	_, err = io.Copy(bufWriter, &ioProxyReader{reader: res.Body, send: func(n int) {
		select {
		case <-timeoutCtx.Done():
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
		return fmt.Errorf("context 关闭")
	case <-timeoutCtx.Done():
		return fmt.Errorf("超时")
	default:
		if err != nil {
			return fmt.Errorf("io.Copy: %s", err)
		}
		operat.finishHook()
	}
	return nil
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
func (operat *operation) sendStat(ctx context.Context, groupPool *WaitGroupPool) {
	oldCompletedLength := atomic.LoadInt64(operat.stat.CompletedLength)
Loop:
	for {
		select {
		case <-ctx.Done():
			break Loop
		default:
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
		}
		time.Sleep(time.Second)
	}
}

// init 初始化，down 配置的应用
func (operat *operation) init() {
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

}

// request 对于 http.NewRequestWithContext 的包装
func (operat *operation) request(method string, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(operat.ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	header := make(http.Header, len(operat.meta.Header))

	for k, v := range operat.meta.Header {
		tmpVal := make([]string, len(v))
		copy(tmpVal, v)
		header[k] = v
	}

	req.Header = header

	return req, nil
}

// do 对于 client.Do 的包装，主要实现重试机制
func (operat *operation) do(rsequest *http.Request) (*http.Response, error) {
	// 请求失败时，重试机制
	var (
		res          *http.Response
		requestError error
		retryNum     = 0
	)
	for ; ; retryNum++ {
		res, requestError = operat.client.Do(rsequest)
		if requestError == nil && res.StatusCode < 400 {
			break
		} else if retryNum+1 >= operat.down.RetryNumber {
			var err error
			if requestError != nil {
				err = fmt.Errorf("down error: %v", requestError)
			} else {
				err = fmt.Errorf("down error: %s HTTP %d", operat.meta.URI, res.StatusCode)
			}
			return nil, err
		}
		time.Sleep(operat.down.RetryTime)
	}
	return res, nil

}

// baseInfo 获取资源基础信息，多线程支持的判断
func (operat *operation) baseInfo() error {
	req, err := operat.request(operat.meta.Method, operat.meta.URI, operat.meta.Body)
	if err != nil {
		return err
	}

	req.Header.Set("range", "bytes=0-9")

	res, err := operat.do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	contentRange := res.Header.Get("content-range")

	rangeList := strings.Split(contentRange, "/")

	if len(rangeList) > 1 {
		operat.size, _ = strconv.ParseInt(rangeList[1], 10, 64)
	}
	// 文件名称
	contentDisposition := res.Header.Get("content-disposition")
	contentType := res.Header.Get("content-type")

	// 是否可以使用多线程
	if res.Header.Get("accept-ranges") != "" || strings.Contains(contentRange, "bytes") || res.Header.Get("content-length") == "10" {
		headinfo, _ := io.ReadAll(res.Body)
		operat.filename = getFileName(operat.meta.URI, contentDisposition, contentType, headinfo)
		operat.multithread = true
	} else {
		// 没有获取到 size ，大概率是因为不支持范围获取数据
		if operat.size == 0 {
			operat.size, _ = strconv.ParseInt(res.Header.Get("content-length"), 10, 64)
		}
		operat.filename = getFileName(operat.meta.URI, contentDisposition, contentType, []byte{})
	}

	return nil
}
