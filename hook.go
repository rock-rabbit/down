package down

// PerHook 是用来创建 Hook 的接口
// down 会在下载之前执行 Make 获得 Hook
// PerHook 的存在是为了在每次执行下载时获取新的 Hook, 不然所有下载都会共用一个 Hook
type PerHook interface {
	Make(stat *Stat) (Hook, error)
}

type Hook interface {
	Send(*Stat) error
	Finish(error, *Stat) error
}

type Hooks []Hook

func (hooks Hooks) Send(stat *Stat) error {
	var err error
	for _, hook := range hooks {
		err = hook.Send(stat)
		if err != nil {
			return err
		}
	}
	return nil
}

func (hooks Hooks) Finish(downerr error, stat *Stat) error {
	var err error
	for _, hook := range hooks {
		err = hook.Finish(downerr, stat)
		if err != nil {
			return err
		}
	}
	return nil
}
