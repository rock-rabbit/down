package down

import "fmt"

// BarTemplate 进度条显示内容的参数
type BarTemplate struct {
	// Template

	// Head 是进度条显示前缀
	Head string

	// End 是进度条显示后缀
	End string
}

// BarHook 提供一个简单的进度条 Hook
type BarHook struct {
	// Template 进度条样式
	Template *BarTemplate

	// Hide 是否隐藏进度条
	Hide bool
}

// Make 初始化 Hook
func (*BarHook) Make(stat *Stat) Hook {
	return &BarHook{
		Template: &BarTemplate{
			Head: "down",
			End:  "",
		},
		Hide: false,
	}
}

// Send 接收数据
func (*BarHook) Send(stat *Stat) error {
	fmt.Println(stat)
	return nil
}
