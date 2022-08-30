package down

import (
	"context"
	"crypto/tls"
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

type closeFunc func()

// operatDown 单个下载请求的处理
type operatDown struct {
	// meta 请求信息
	meta *Meta

	// config 下载配置
	config *Down

	// client
	client *http.Client

	// wgpool 线程池
	wgpool *WaitGroupPool

	// multithread 是否使用多线程下载
	multithread bool

	// breakpoint 是否使用断点续传
	breakpoint bool

	// filesize 文件大小
	filesize int64

	// cl 已下载的大小
	cl *int64

	// filename 从 URI 和 头信息 中获得的文件名称, 未指定名称时使用
	filename string

	// outpath 下载目标位置
	outpath string

	// ctlpath 控制文件位置
	ctlpath string

	// operatFile 操作文件
	operatFile *operatFile

	// done
	done chan error

	err error

	// close 关闭
	close closeFunc
}

func (od *operatDown) init(ctx context.Context) error {
	// 创建上下文
	ctx, cancel := context.WithCancel(ctx)
	od.close = func() { cancel() }
	// 请求配置
	od.client = &http.Client{
		Transport: &http.Transport{
			// 应用来自环境变量的代理
			Proxy: od.config.Proxy,
			// 要求服务器返回非压缩的内容，前提是没有发送 accept-encoding 来接管 transport 的自动处理
			DisableCompression: true,
			// 等待响应头的超时时间
			ResponseHeaderTimeout: od.config.ConnectTimeout,
			// TLS 握手超时时间
			TLSHandshakeTimeout: 10 * time.Second,
			// 接受服务器提供的任何证书
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		// 超时时间
		Timeout: 0,
	}
	od.cl = new(int64)
	od.done = make(chan error)
	od.wgpool = NewWaitGroupPool(od.config.ThreadCount)

	// 检查远程资源和本地文件
	if err := od.check(ctx); err != nil {
		return err
	}
	return nil
}

// start 开始执行下载
func (od *operatDown) start(ctx context.Context) {
	go od.electe(ctx)
}

func (od *operatDown) electe(ctx context.Context) {
	// 单线程下载逻辑
	if !od.multithread || od.config.ThreadCount <= 1 {
		if od.breakpoint {
			od.singleBreakpoint(ctx)
		} else {
			od.single(ctx)
		}
		return
	}

	// 多线程下载逻辑
	if od.breakpoint {
		od.multithBreakpoint(ctx)
	} else {
		od.multith(ctx)
	}
}

// finish 下载完成
func (od *operatDown) finish(err error) {
	// 释放资源
	od.close()
	od.operatFile.close()

	od.err = err
	if err == nil {
		// 删除控制文件
		od.operatFile.operatCF.remove()
	}
	od.done <- err
}

// wait 等待下载完成
func (od *operatDown) wait() error {
	return <-od.done
}

// check 检查远程资源和本地文件
func (od *operatDown) check(ctx context.Context) error {
	var err error

	// 检查是否可以使用多线程，顺便获取一些数据
	err = od.checkMultith(ctx)
	if err != nil {
		return err
	}

	// 文件位置
	outputName := od.meta.OutputName
	if outputName == "" {
		outputName = od.filename
	}
	od.outpath, err = filepath.Abs(filepath.Join(od.meta.OutputDir, outputName))
	if err != nil {
		return err
	}
	od.ctlpath = fmt.Sprintf("%s.%s", od.outpath, od.config.TempFileExt)

	// 控制文件
	operatCF := newOperatCF(ctx, od.ctlpath)

	// 文件检查
	err = od.checkFile(operatCF)
	if err != nil {
		return err
	}

	// 检查到不需要断点续传，新建控制文件
	if !od.breakpoint {
		err = operatCF.open(od.meta.Perm)
		if err != nil {
			return err
		}
		operatCF.cf = newControlfile(0)
		operatCF.cf.total = od.filesize
	}

	// 创建操作文件
	od.operatFile, err = newOperatFile(ctx, operatCF, od.outpath, od.cl, od.config.DiskCache, od.meta.Perm)
	if err != nil {
		return err
	}
	return nil
}

// checkFile 文件检查
func (od *operatDown) checkFile(operatCF *operatCF) error {
	var err error
	// 文件是否存在, 这里之后支持断点续传后需要改逻辑
	outpathexist := fileExist(od.outpath)
	ctlexist := fileExist(od.ctlpath)

	// 目录不存在时创建目录
	if od.config.CreateDir && !fileExist(od.meta.OutputDir) {
		os.MkdirAll(od.meta.OutputDir, os.ModePerm)
	}

	if od.multithread && outpathexist && od.config.Continue && ctlexist {
		// 控制文件是否可以进行断点续传
		ok, err := operatCF.check(od.meta.Perm)
		if err != nil {
			return err
		}
		if ok && operatCF.cf.total == od.filesize {
			atomic.SwapInt64(od.cl, operatCF.cf.completedLength())
			od.breakpoint = true
			return nil
		}
	}

	if outpathexist && od.config.AllowOverwrite {
		// 允许删除文件重新下载
		err = os.Remove(od.outpath)
		if err != nil {
			return err
		}
	} else if outpathexist {
		return fmt.Errorf(ErrorFileExist, od.outpath)
	}

	return nil
}

// checkMultith 检查是否可以使用多线程，顺便获取一些数据
func (od *operatDown) checkMultith(ctx context.Context) error {
	res, err := od.rangeDo(ctx, 0, 9)
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
		od.filesize, _ = strconv.ParseInt(rangeList[1], 10, 64)
	}

	// 是否可以使用多线程
	if acceptRanges != "" || strings.Contains(contentRange, "bytes") || contentLength == "10" {
		headinfo, _ = io.ReadAll(res.Body)
		od.multithread = true
	} else {
		// 不支持多线程重新获取文件总大小
		if od.filesize == 0 {
			od.filesize, _ = strconv.ParseInt(contentLength, 10, 64)
		}
	}

	// 自动获取文件名称
	od.filename = getFileName(od.meta.URI, contentDisposition, contentType, headinfo)

	return nil
}

// rangeDo 基于 range 的请求
func (od *operatDown) rangeDo(ctx context.Context, start, end int64) (*http.Response, error) {
	res, err := od.defaultDo(ctx, func(req *http.Request) error {
		req.Header.Set("range", fmt.Sprintf("bytes=%d-%d", start, end))
		return nil
	})
	if err != nil {
		return res, err
	}
	return res, nil
}

// defaultDo 基于默认参数的请求
func (od *operatDown) defaultDo(ctx context.Context, call func(req *http.Request) error) (*http.Response, error) {
	req, err := od.request(ctx, http.MethodGet, od.meta.URI, od.meta.Body)
	if err != nil {
		return nil, err
	}
	if call != nil {
		err = call(req)
		if err != nil {
			return nil, err
		}
	}
	res, err := od.do(req)
	if err != nil {
		return res, err
	}
	return res, nil
}

// request 对于 http.NewRequestWithContext 的包装
func (od *operatDown) request(ctx context.Context, method string, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	header := make(http.Header, len(od.meta.Header))

	for k, v := range od.meta.Header {
		tmpVal := make([]string, len(v))
		copy(tmpVal, v)
		header[k] = v
	}

	req.Header = header

	return req, nil
}

// do 对于 client.Do 的包装，主要实现重试机制
func (od *operatDown) do(rsequest *http.Request) (*http.Response, error) {
	// 请求失败时，重试机制
	var (
		res          *http.Response
		requestError error
		retryNum     = 0
	)
	for ; ; retryNum++ {
		res, requestError = od.client.Do(rsequest)
		if requestError == nil && res.StatusCode < 400 {
			break
		} else if retryNum+1 >= od.config.RetryNumber {
			var err error
			if requestError != nil {
				err = requestError
			} else {
				err = fmt.Errorf(ErrorRequestStatus, od.meta.URI, res.StatusCode)
			}
			return nil, err
		}
		time.Sleep(od.config.RetryTime)
	}
	return res, nil

}
