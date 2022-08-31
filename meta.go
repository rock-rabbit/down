package down

import (
	"io"
	"io/fs"
	"net/http"
)

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

// defaultHeader 默认请求头
var defaultHeader = http.Header{
	"accept":          []string{"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9"},
	"accept-language": []string{"zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6"},
	"user-agent":      []string{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/104.0.5112.81 Safari/537.36 Edg/104.0.1293.54"},
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

// SimpleMeta 用简单的方式创建一个 Meta
func SimpleMeta(s ...string) *Meta {
	var uri, outpath, filename string
	switch len(s) {
	case 1:
		uri = s[0]
	case 2:
		uri = s[0]
		outpath = s[1]
	default:
		uri = s[0]
		outpath = s[1]
		filename = s[2]
	}
	return NewMeta(uri, outpath, filename)
}

// Copy 在执行下载前，会拷贝 Meta
func (meta *Meta) Copy() *Meta {
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
