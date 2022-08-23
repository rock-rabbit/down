package down

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
	"time"
)

// BarTemplate 进度条显示内容的参数
type BarTemplate struct {
	// Template 模版
	Template string

	// NoSizeTemplate 获取不到文件大小时的模板
	NoSizeTemplate string

	// template 模版
	template *template.Template

	// Saucer 进度字符, 默认为 =
	Saucer string

	// SaucerHead 进度条头, 使用场景 =====> , 其中的 > 就是进度条头, 默认为 >
	SaucerHead string

	// SaucerPadding 进度空白字符, 默认为 -
	SaucerPadding string

	// BarStart 进度前缀, 默认为 [
	BarStart string

	// BarEnd 进度后缀, 默认为 ]
	BarEnd string

	// BarWidth 进度条宽度, 默认为 80
	BarWidth int
}

// BarStatString 注入到模版中的字符串结构
// {{.CompletedLength}} / {{.TotalLength}} {{.Saucer}} {{.Progress}}% {{.DownloadSpeed}}/s {{.EstimatedTime}}
type BarStatString struct {
	// TotalLength 文件总大小
	TotalLength string

	// CompletedLength 已下载大小
	CompletedLength string

	// DownloadSpeed 文件每秒下载速度
	DownloadSpeed string

	// ConsumingTime 耗时
	ConsumingTime string

	// EstimatedTime 预计下载完成还需要的时间
	EstimatedTime string

	// Progress 下载进度, 长度为 100
	Progress string

	// Saucer 进度条
	Saucer string

	// Connections 与服务器的连接数
	Connections string
}

// BarStat 进度条信息
type BarStat struct {
	// TotalLength 文件总大小
	TotalLength int64

	// CompletedLength 已下载大小
	CompletedLength int64

	// DownloadSpeed 文件每秒下载速度
	DownloadSpeed int64

	// StartTime 开始时间
	StartTime time.Time

	// EstimatedTime 预计下载完成还需要的时间
	EstimatedTime time.Duration

	// Progress 下载进度, 长度为 100
	Progress int

	// Connections 与服务器的连接数
	Connections int
}

// BarHook 提供一个简单的进度条 Hook
type BarHook struct {
	// Template 进度条样式
	Template *BarTemplate

	// FriendlyFormat 使用人类友好的单位
	FriendlyFormat bool

	// FinishHide 完成后隐藏进度条,下载完成后清除掉进度条
	FinishHide bool

	// Hide 是否隐藏进度条
	Hide bool

	// Stdout 进度条输出, 默认为 os.Stdout
	Stdout io.Writer

	// finish 进度条是否已完成
	finish bool

	// stat 进度条渲染时包含的数据
	stat *BarStat
}

var DefaultBarHook = &BarHook{
	Template: &BarTemplate{
		Template:       `{{.CompletedLength}} / {{.TotalLength}} {{.Saucer}} {{.Progress}}% {{.DownloadSpeed}}/s {{.EstimatedTime}} CN:{{.Connections}}`,
		NoSizeTemplate: `{{.CompletedLength}} {{.DownloadSpeed}}/s {{.ConsumingTime}}`,
		Saucer:         "=",
		SaucerHead:     ">",
		SaucerPadding:  "-",
		BarStart:       "[",
		BarEnd:         "]",
		BarWidth:       80,
	},
	FriendlyFormat: true,
	Hide:           false,
	Stdout:         os.Stdout,
	FinishHide:     false,
}

// Make 初始化 Hook
func (barhook *BarHook) Make(stat *Stat) (Hook, error) {
	// 拷贝结构
	tmpBarhook := *barhook
	tmpBarhook.stat = new(BarStat)
	tmpBarhook.stat.StartTime = time.Now()
	tmpl := *barhook.Template
	tmpBarhook.Template = &tmpl

	// 解析模版
	var err error
	if stat.TotalLength == 0 {
		tmpBarhook.Template.template, err = template.New("downBarTemplate").Parse(tmpBarhook.Template.NoSizeTemplate)
	} else {
		tmpBarhook.Template.template, err = template.New("downBarTemplate").Parse(tmpBarhook.Template.Template)
	}
	if err != nil {
		return nil, err
	}

	// 首次处理数据
	tmpBarhook.receiveStat(stat)
	return &tmpBarhook, nil
}

// Send 接收数据
func (barhook *BarHook) Send(stat *Stat) error {
	// 隐藏进度条
	if barhook.Hide {
		return nil
	}

	// 处理数据
	barhook.receiveStat(stat)

	// 渲染进度条
	if err := barhook.render(); err != nil {
		return err
	}
	return nil
}

// render 渲染
func (barhook *BarHook) render() error {
	stat := *barhook.stat
	// 是否使用人性化格式
	formatFileSizeFunc := func(fileSize int64) string {
		return fmt.Sprintf("%d B", fileSize)
	}
	formatTimeFunc := func(t time.Duration) string {
		return fmt.Sprintf("%ds", int(t.Seconds()))
	}
	if barhook.FriendlyFormat {
		formatFileSizeFunc = formatFileSize
		formatTimeFunc = func(t time.Duration) string {
			return fmt.Sprintf("%v", t)
		}
	}

	// 将数据转为字符串结构
	statString := BarStatString{
		// TotalLength 文件总大小
		TotalLength: formatFileSizeFunc(stat.TotalLength),

		// CompletedLength 已下载大小
		CompletedLength: formatFileSizeFunc(stat.CompletedLength),

		// DownloadSpeed 文件每秒下载速度
		DownloadSpeed: formatFileSizeFunc(stat.DownloadSpeed),

		// ConsumingTime 耗时
		ConsumingTime: formatTimeFunc(time.Duration(int(time.Since(stat.StartTime).Seconds()) * int(time.Second))),

		// EstimatedTime 预计下载完成还需要的时间
		EstimatedTime: formatTimeFunc(stat.EstimatedTime),

		// Progress 下载进度, 长度为 100
		Progress: fmt.Sprint(stat.Progress),

		// Connections 与服务器的连接数
		Connections: fmt.Sprint(stat.Connections),

		// Saucer 这里使用 _____Saucer_____ 占位置, 长度 16
		Saucer: "_____Saucer_____",
	}
	// 模版渲染
	barTemplate := bytes.NewBuffer(make([]byte, 0))
	err := barhook.Template.template.Execute(barTemplate, statString)
	if err != nil {
		return err
	}
	barTemplateString := barTemplate.String()
	// 模版中是否存在占位置的 Saucer
	saucerlength := 0
	if strings.Contains(barTemplateString, statString.Saucer) {
		saucerlength = 16
	}
	// 计算进度条需要占用的长度
	barStart := barhook.Template.BarStart
	barEnd := barhook.Template.BarEnd
	width := barhook.Template.BarWidth - len(barTemplateString) - len(barStart) - len(barEnd) + saucerlength
	saucerCount := int(float64(stat.Progress) / 100.0 * float64(width))
	// 组装进度条
	saucerBuffer := bytes.NewBuffer(make([]byte, 0))
	if saucerCount > 0 {
		saucerBuffer.WriteString(barStart)
		saucerBuffer.WriteString(strings.Repeat(barhook.Template.Saucer, saucerCount-1))

		saucerHead := barhook.Template.SaucerHead
		if saucerHead == "" || barhook.finish {
			saucerHead = barhook.Template.Saucer
		}
		saucerBuffer.WriteString(saucerHead)

		saucerBuffer.WriteString(strings.Repeat(barhook.Template.SaucerPadding, width-saucerCount))

		saucerBuffer.WriteString(barEnd)
	} else {
		saucerBuffer.WriteString(barStart)
		saucerBuffer.WriteString(strings.Repeat(barhook.Template.SaucerPadding, width))
		saucerBuffer.WriteString(barEnd)
	}
	// 替换占位的进度条并打印
	fmt.Fprintf(barhook.Stdout, "\r%s", strings.ReplaceAll(barTemplateString, statString.Saucer, saucerBuffer.String()))
	return nil
}

// Finish 完成后的渲染, 会将进度设为 100%
func (barhook *BarHook) Finish(stat *Stat) error {
	if barhook.Hide {
		return nil
	}
	// 下载完成后清除进度条
	if barhook.FinishHide {
		fmt.Printf("\r%s\r", strings.Repeat(" ", barhook.Template.BarWidth))
		return nil
	}
	barhook.finish = true
	barhook.stat.CompletedLength = barhook.stat.TotalLength
	barhook.stat.Progress = 100
	barhook.render()
	fmt.Println()
	return nil
}

// receiveStat 处理 Stat
func (barhook *BarHook) receiveStat(stat *Stat) {
	barhook.stat.TotalLength = stat.TotalLength
	barhook.stat.CompletedLength = stat.CompletedLength
	barhook.stat.DownloadSpeed = stat.DownloadSpeed
	barhook.stat.Connections = stat.Connections
	// 计算进度
	if barhook.stat.TotalLength > 0 && barhook.stat.CompletedLength > 0 {
		barhook.stat.Progress = int(float64(barhook.stat.CompletedLength) / float64(barhook.stat.TotalLength) * float64(100))
	}
	// 计算下载完成大约还需要的时间
	remainingLength := barhook.stat.TotalLength - barhook.stat.CompletedLength
	if barhook.stat.DownloadSpeed <= 0 || remainingLength <= 0 {
		return
	}
	barhook.stat.EstimatedTime = time.Duration((remainingLength / barhook.stat.DownloadSpeed) * int64(time.Second))
}
