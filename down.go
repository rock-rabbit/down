package down

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/fs"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

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
	// Connections 与资源服务器的连接数
	Connections int
}

// stating 下载进行时的数据
type stating struct {
	CompletedLength *int64
}

// Meta 下载信息，请求信息和存储信息
type Meta struct {
	// URI 下载资源的地址
	URI string
	// OutputName 输出文件名，为空则通过 getFileName 自动获取
	OutputName string
	// OutputDir 输出目录，默认为 ./
	OutputDir string

	// Method 默认为 GET
	Method string

	// Body 请求时的 Body，默认为 nil
	Body io.Reader

	// Header 请求头，默认拷贝 defaultHeader
	Header http.Header

	// Perm 新建文件的权限, 默认为 0600
	Perm fs.FileMode
}

// Down 下载器，请求配置和 Hook 信息
type Down struct {
	// PerHooks 是返回下载进度的钩子
	PerHooks []PerHook
	// ThreadCount 多线程下载时最多同时下载一个文件的线程
	ThreadCount int
	// ThreadSize 多线程下载时每个线程下载的字节数
	ThreadSize int64
	// CreateDir 当需要创建目录时，是否创建目录，默认为 true
	CreateDir bool
	// Replace 遇到相同文件时是否要强制替换
	Replace bool
	// Resume 是否每次都重新下载,不尝试断点续传
	Resume bool
	// ConnectTimeout HTTP 连接请求的超时时间
	ConnectTimeout time.Duration
	// Timeout 超时时间
	Timeout time.Duration
	// RetryNumber 最多重试次数
	RetryNumber int
	// RetryTime 重试时的间隔时间
	RetryTime time.Duration
	// Proxy Http 代理设置
	Proxy func(*http.Request) (*url.URL, error)
	// TempFileExt 临时文件后缀, 默认为 down
	TempFileExt string
	// mux 锁
	mux sync.Mutex
}

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
	// ctx 上下文
	ctx context.Context

	// stat 下载进行时的进度记录
	stat *stating
}

var (
	// Default 默认下载器
	Default = New()

	// defaultHeader 默认请求头
	defaultHeader = http.Header{
		"accept":          []string{"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9"},
		"accept-encoding": []string{"gzip, deflate, br"},
		"accept-language": []string{"zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6"},
		"user-agent":      []string{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/104.0.5112.81 Safari/537.36 Edg/104.0.1293.54"},
	}
)

// New 创建一个默认的下载器
func New() *Down {
	return &Down{
		PerHooks:       make([]PerHook, 0),
		ThreadCount:    1,
		ThreadSize:     1048576,
		CreateDir:      true,
		Replace:        true,
		Resume:         true,
		ConnectTimeout: time.Second * 60,
		Timeout:        time.Second * 60,
		RetryNumber:    5,
		RetryTime:      0,
		Proxy:          http.ProxyFromEnvironment,
		TempFileExt:    "down",
		mux:            sync.Mutex{},
	}
}

// NewMeta 创建一个新的 Meta
func NewMeta(uri, outputDir, outputName string) *Meta {
	header := make(http.Header, len(defaultHeader))

	for k, v := range defaultHeader {
		tmpVal := make([]string, len(v))
		copy(tmpVal, v)
		header[k] = v
	}

	return &Meta{
		URI:        uri,
		OutputName: outputName,
		OutputDir:  outputDir,
		Method:     http.MethodGet,
		Body:       nil,
		Header:     header,
		Perm:       0600,
	}
}

// copy 在执行下载前，会拷贝 Meta
func (meta *Meta) copy() *Meta {
	tmpMeta := *meta

	header := make(http.Header, len(meta.Header))

	for k, v := range meta.Header {
		tmpVal := make([]string, len(v))
		copy(tmpVal, v)
		header[k] = v
	}

	tmpMeta.Header = header

	return &tmpMeta
}

// AddHook 添加 Hook 的创建接口
func (down *Down) AddHook(perhook PerHook) {
	down.mux.Lock()
	defer down.mux.Unlock()
	down.PerHooks = append(down.PerHooks, perhook)
}

// copy 在执行下载前，会拷贝 Down
func (down *Down) copy() *Down {
	tmpDown := *down
	tmpDown.PerHooks = make([]PerHook, len(down.PerHooks))
	copy(tmpDown.PerHooks, down.PerHooks)

	tmpDown.mux = sync.Mutex{}
	return &tmpDown
}

// Run 执行下载
func (down *Down) Run(meta *Meta) error {
	return down.RunContext(context.Background(), meta)
}

// RunContext 基于 context 执行下载
func (down *Down) RunContext(ctx context.Context, meta *Meta) error {
	var ins *operation
	// 组合操作结构,将配置拷贝一份
	down.mux.Lock()
	ins = &operation{
		down: down.copy(),
		meta: meta.copy(),
		stat: &stating{
			CompletedLength: new(int64),
		},
		ctx: ctx,
	}
	down.mux.Unlock()
	// 生成 Hook
	ins.hooks = make([]Hook, len(ins.down.PerHooks))
	stat := &Stat{Down: ins.down, Meta: ins.meta}
	var err error
	for idx, perhook := range ins.down.PerHooks {
		ins.hooks[idx], err = perhook.Make(stat)
		if err != nil {
			return fmt.Errorf("down error: Make Hook: %s", err)
		}
	}
	// 初始化操作
	ins.init()
	// 读取文件基础信息
	ins.baseInfo()
	// 文件存储路径
	var (
		outputName, outputPath string
	)
	outputName = meta.OutputName
	if outputName == "" {
		outputName = ins.filename
	}
	outputPath, err = filepath.Abs(filepath.Join(ins.meta.OutputDir, outputName))
	if err != nil {
		return fmt.Errorf("down error: filepath.Abs: %s", err)
	}
	// 文件是否存在, 这里之后支持断点续传后需要改逻辑
	if fileExist(outputPath) {
		if !ins.down.Replace {
			return fmt.Errorf("down error: 已存在文件 %s，若要强制替换文件请将 down.Replace 设为 true", outputPath)
		}
		// 需要强制覆盖, 删除掉原文件
		err = os.Remove(outputPath)
		if err != nil {
			return fmt.Errorf("down error: Remove file: %s", err)
		}
	}
	// 目录不存在时创建目录
	if ins.down.CreateDir && !fileExist(ins.meta.OutputDir) {
		os.MkdirAll(ins.meta.OutputDir, os.ModePerm)
	}
	// 单线程下载逻辑
	if !ins.multithread || ins.down.ThreadCount <= 1 {
		f, err := os.OpenFile(outputPath, os.O_CREATE|os.O_RDWR, ins.meta.Perm)
		if err != nil {
			return fmt.Errorf("down error: Open file: %s", err)
		}
		defer f.Close()

		if err := f.Truncate(ins.size); err != nil {
			return err
		}

		req, err := ins.request(http.MethodGet, ins.meta.URI, nil)
		if err != nil {
			return fmt.Errorf("down error: Request: %s", err)
		}
		res, err := ins.do(req)
		if err != nil {
			return fmt.Errorf("down error: Request Do: %s", err)
		}
		defer res.Body.Close()

		// 当基础信息阶段没有获取到文件大小, 从这里再读取一遍
		if ins.size == 0 {
			ins.size, _ = strconv.ParseInt(res.Header.Get("content-length"), 10, 64)
		}

		// 每秒给 Hook 发送信息
		go ins.sendStat(ctx, 1)
		// 使用代理 io 写入文件
		_, err = io.Copy(f, &ioProxyReader{reader: res.Body, send: func(n int) {
			select {
			case <-ctx.Done():
				res.Body.Close()
			default:
				atomic.AddInt64(ins.stat.CompletedLength, int64(n))
			}
		}})
		if err != nil {
			return fmt.Errorf("down error: io.Copy: %s", err)
		}
		ins.finishHook()
		return nil
	}
	// 多线程下载逻辑
	f, err := os.OpenFile(outputPath, os.O_CREATE|os.O_RDWR, ins.meta.Perm)
	if err != nil {
		return fmt.Errorf("down error: Open file: %s", err)
	}
	defer f.Close()

	if err := f.Truncate(ins.size); err != nil {
		return err
	}
	// 拆分任务
	task := threadTaskSplit(ins.size, ins.down.ThreadSize)
	// 任务执行
	groupPool := NewWaitGroupPool(ins.down.ThreadCount)
	done := make(chan int)
	cherr := make(chan error)
	// 每秒给 Hook 发送信息
	go ins.sendStat(ctx, 1)
	go func() {
		for _, fileRange := range task {
			groupPool.Add()
			go ins.threadTask(groupPool, cherr, f, fileRange[0], fileRange[1])
		}
		groupPool.Wait()
		done <- 1
	}()

Loop:
	select {
	case err = <-cherr:
		return err
	case <-done:
		break Loop
	}
	ins.finishHook()
	return nil
}

// threadTask 单个线程的下载逻辑
func (operat *operation) threadTask(groupPool *WaitGroupPool, cherr chan error, f *os.File, rangeStart, rangeEnd int64) {
	defer groupPool.Done()
	req, err := operat.request(http.MethodGet, operat.meta.URI, operat.meta.Body)
	if err != nil {
		cherr <- fmt.Errorf("down error: Request: %s", err)
		return
	}
	req.Header.Set("range", fmt.Sprintf("bytes=%d-%d", rangeStart, rangeEnd))
	res, err := operat.do(req)
	if err != nil {
		cherr <- fmt.Errorf("down error: Request Do: %s", err)
		return
	}
	defer res.Body.Close()
	bufSize := (rangeEnd - rangeStart) + 1

	buf := bytes.NewBuffer(make([]byte, 0, bufSize))
	// 使用代理 io 写入文件
	_, err = io.Copy(buf, &ioProxyReader{reader: res.Body, send: func(n int) {
		select {
		case <-operat.ctx.Done():
			res.Body.Close()
		default:
			atomic.AddInt64(operat.stat.CompletedLength, int64(n))
		}
	}})
	if err != nil {
		cherr <- fmt.Errorf("down error: io.Copy: %s", err)
		return
	}
	// 写入到文件
	n, err := f.WriteAt(buf.Bytes(), rangeStart)
	if err != nil {
		cherr <- err
		return
	}
	if int64(n) != bufSize {
		cherr <- fmt.Errorf("down error: bytes=%d-%d 写入数据为 %d 字节，与预计 %d 字节不符", rangeStart, rangeEnd, n, bufSize)
		return
	}
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
func (operat *operation) sendStat(ctx context.Context, connections int) {
	oldCompletedLength := atomic.LoadInt64(operat.stat.CompletedLength)
Loop:
	for {
		select {
		case <-ctx.Done():
			break Loop
		default:
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
			Proxy:              operat.down.Proxy,
			DisableCompression: true,
			// TLS 握手超时时间
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 15 * time.Minute,
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
	headinfo, _ := io.ReadAll(res.Body)
	res.Body.Close()

	contentRange := res.Header.Get("content-range")

	rangeList := strings.Split(contentRange, "/")

	if len(rangeList) > 1 {
		operat.size, _ = strconv.ParseInt(rangeList[1], 10, 64)
	}
	// 文件名称
	contentDisposition := res.Header.Get("content-disposition")
	contentType := res.Header.Get("content-type")
	operat.filename = getFileName(operat.meta.URI, contentDisposition, contentType, headinfo)

	// 是否可以使用多线程
	if res.Header.Get("accept-ranges") != "" || strings.Contains(contentRange, "bytes") || res.Header.Get("content-length") == "10" {
		operat.multithread = true
	}

	return nil
}

// threadTask 多线程任务分割
func threadTaskSplit(size, threadSize int64) [][2]int64 {
	// size - 1 是因为范围是从 0 开始，需要提前减去
	size = size - 1

	taskCountFloat64 := float64(size) / float64(threadSize)
	if math.Trunc(taskCountFloat64) != taskCountFloat64 {
		taskCountFloat64++
	}
	taskCount := int(taskCountFloat64)
	task := make([][2]int64, int(taskCount))
	for i := 0; i < taskCount; i++ {
		if i == 0 {
			task[i][0] = int64(i) * threadSize
		} else {
			task[i][0] = int64(i)*threadSize + 1
		}
		task[i][1] = (int64(i) + 1) * threadSize
		if task[i][1] > size {
			task[i][1] = size
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
