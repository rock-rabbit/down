package down

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
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

type Stat struct {
	Meta            *Meta
	Down            *Down
	TotalLength     int64
	CompletedLength int64
	DownloadSpeed   int64
	Connections     int
}

// stating 下载进行时的数据
type stating struct {
	CompletedLength *int64
}

type Meta struct {
	URI        string
	OutputName string
	OutputDir  string
	Header     http.Header
	// Perm 新建文件的权限, 默认为 0600
	Perm fs.FileMode
}

type Down struct {
	// PerHooks 是返回下载进度的钩子
	PerHooks []PerHook
	// ThreadCount 多线程下载时最多同时下载一个文件的线程
	ThreadCount int
	// ThreadSize 多线程下载时每个线程下载的字节数
	ThreadSize int64
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
	down   *Down
	meta   *Meta
	hooks  []Hook
	client *http.Client
	// size 文件大小
	size int64
	// multithread 是否使用多线程下载
	multithread bool
	// filename 从 URI 和 头信息 中获得的文件名称, 未指定名称时使用
	filename string

	stat *stating
}

var (
	Default       = New()
	defaultHeader = http.Header{
		"accept":          []string{"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9"},
		"accept-encoding": []string{"gzip, deflate, br"},
		"accept-language": []string{"zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6"},
		"user-agent":      []string{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/104.0.5112.81 Safari/537.36 Edg/104.0.1293.54"},
	}
)

func New() *Down {
	return &Down{
		PerHooks:       make([]PerHook, 0),
		ThreadCount:    1,
		ThreadSize:     4194304,
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
		Header:     header,
		Perm:       0600,
	}
}

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

func (down *Down) AddHook(perhook PerHook) {
	down.mux.Lock()
	defer down.mux.Unlock()
	down.PerHooks = append(down.PerHooks, perhook)
}

func (down *Down) copy() *Down {
	tmpDown := *down
	tmpDown.PerHooks = make([]PerHook, len(down.PerHooks))
	copy(tmpDown.PerHooks, down.PerHooks)

	tmpDown.mux = sync.Mutex{}
	return &tmpDown
}

func (down *Down) Run(meta *Meta) error {
	return down.RunContext(context.Background(), meta)
}

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
	}
	down.mux.Unlock()
	// 生成 Hook
	ins.hooks = make([]Hook, len(ins.down.PerHooks))
	stat := &Stat{Down: ins.down, Meta: ins.meta}
	for idx, perhook := range ins.down.PerHooks {
		ins.hooks[idx] = perhook.Make(stat)
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
	outputPath, err := filepath.Abs(filepath.Join(meta.OutputDir, outputName))
	if err != nil {
		return err
	}
	// 文件是否存在, 这里之后支持断点续传后需要改逻辑
	if fileExist(outputPath) {
		if !ins.down.Replace {
			return fmt.Errorf("down: 已存在文件 %s，若要强制替换文件请将 down.Replace 设为 true", outputPath)
		}
		// 需要强制覆盖, 删除掉原文件
		err = os.Remove(outputPath)
		if err != nil {
			return err
		}
	}
	// 单线程下载逻辑
	if !ins.multithread || ins.down.ThreadCount <= 1 {
		f, err := os.OpenFile(outputPath, os.O_CREATE|os.O_RDWR, ins.meta.Perm)
		if err != nil {
			return err
		}
		defer f.Close()

		req, err := ins.request(http.MethodGet, ins.meta.URI, nil)
		if err != nil {
			return err
		}
		res, err := ins.do(req)
		if err != nil {
			return err
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
			return err
		}
		return nil
	}

	return nil
}

func (operat *operation) sendHook(stat *Stat) error {
	var tmpHooks []Hook
	operat.down.mux.Lock()
	tmpHooks = make([]Hook, len(operat.hooks))
	copy(tmpHooks, operat.hooks)
	operat.down.mux.Unlock()

	err := Hooks(tmpHooks).Send(stat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "send hook 失败: %v\n", err)
	}
	return nil
}

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

func (operat *operation) request(method string, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
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
				err = fmt.Errorf("%s down error: HTTP %d", operat.meta.URI, res.StatusCode)
			}
			return nil, err
		}
		time.Sleep(operat.down.RetryTime)
	}
	return res, nil

}

func (operat *operation) baseInfo() error {
	req, err := operat.request(http.MethodGet, operat.meta.URI, nil)
	if err != nil {
		return err
	}

	req.Header.Set("range", "bytes=0-9")

	res, err := operat.do(req)
	if err != nil {
		return err
	}
	headinfo, _ := ioutil.ReadAll(res.Body)
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
		if len(us) > 0 {
			name = us[len(us)-1]
		}
	}
	// 尝试在文件魔数获取文件后缀
	fileType := getFileType(headinfo)
	if fileType != "" {
		ext = fmt.Sprintf(".%s", fileType)
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
