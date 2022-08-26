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
	"time"
)

// init 初始化，down 配置的应用
func (operat *operation) init() error {
	var err error

	// 应用配置
	operat.useconfig()

	// 检查是否可以使用多线程，顺便获取一些数据
	err = operat.checkMultith()
	if err != nil {
		return err
	}

	// 文件存储路径
	err = operat.usefilepath()
	if err != nil {
		return err
	}

	// 文件检查
	err = operat.checkFile()
	if err != nil {
		return err
	}

	// 创建 Hook
	err = operat.makeHook()
	if err != nil {
		return err
	}

	// 创建上下文
	if operat.down.Timeout <= 0 {
		operat.ctx, operat.ctxCance = context.WithCancel(operat.ctx)
	} else {
		// 创建超时 context
		operat.ctx, operat.ctxCance = context.WithTimeout(operat.ctx, operat.down.Timeout)
	}

	// 创建操作文件
	operat.operatFile, err = newOperatFile(operat.ctx, operat.outputPath, operat.meta.Perm, operat.down.DiskCache)
	if err != nil {
		return err
	}
	operat.operatFile.cl = operat.stat.CompletedLength

	return nil
}

// makeHook 创建 Hook
func (operat *operation) makeHook() error {
	var err error
	operat.hooks = make([]Hook, len(operat.down.PerHooks))
	stat := &Stat{Down: operat.down, Meta: operat.meta, TotalLength: operat.size, OutputPath: operat.outputPath}
	for idx, perhook := range operat.down.PerHooks {
		operat.hooks[idx], err = perhook.Make(stat)
		if err != nil {
			return fmt.Errorf("Make Hook: %s", err)
		}
	}
	return nil
}

// checkFile 文件检查
func (operat *operation) checkFile() error {
	var err error
	// 文件是否存在, 这里之后支持断点续传后需要改逻辑
	outputPathExist := fileExist(operat.outputPath)

	// 目录不存在时创建目录
	if operat.down.CreateDir && !fileExist(operat.meta.OutputDir) {
		os.MkdirAll(operat.meta.OutputDir, os.ModePerm)
	}

	operat.operatCF = newOperatCF()
	if outputPathExist && operat.down.Continue && fileExist(operat.controlfilePath) {
		// 可以使用断点下载 并且 存在控制文件
		err = operat.operatCF.open(operat.controlfilePath, operat.meta.Perm)
		if err != nil {
			return err
		}
		err = operat.operatCF.read()
		if err != nil {
			return err
		}
		if operat.operatCF.getCF() != nil {
			operat.breakpoint = true
			return nil
		}
		// 控制文件损坏，不能使用断点续传
	}

	operat.operatCF.newControlfile()

	if outputPathExist && operat.down.AllowOverwrite {
		// 允许删除文件重新下载
		err = os.Remove(operat.outputPath)
		if err != nil {
			return err
		}
		// 删除控制文件
		os.Remove(operat.controlfilePath)
	} else if outputPathExist {
		return fmt.Errorf(ErrorFileExist, operat.outputPath)
	}

	return nil
}

// usefilepath 应用文件路径
func (operat *operation) usefilepath() error {
	var err error
	outputName := operat.meta.OutputName
	if outputName == "" {
		outputName = operat.filename
	}
	operat.outputPath, err = filepath.Abs(filepath.Join(operat.meta.OutputDir, outputName))
	if err != nil {
		return err
	}
	operat.controlfilePath = fmt.Sprintf("%s.%s", operat.outputPath, operat.down.TempFileExt)
	return nil
}

// useconfig 应用配置
func (operat *operation) useconfig() {
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

// checkMultith 检查是否可以使用多线程，顺便获取一些数据
func (operat *operation) checkMultith() error {
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
