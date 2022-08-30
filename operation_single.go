package down

import "context"

// single 单线程，非断点续传
func (od *operatDown) single(ctx context.Context) {
	if err := od.operatFile.file.Truncate(od.filesize); err != nil {
		od.finish(err)
		return
	}
	// 自动处理
	go od.operatFile.operatCF.autoSave(od.config.AutoSaveTnterval)
	// 执行下载任务
	od.operatFile.operatCF.addTreadblock(0, 0, od.filesize-1)
	res, err := od.defaultDo(ctx, nil)
	if err != nil {
		od.finish(err)
		return
	}
	defer res.Body.Close()

	// 写入文件
	err = od.operatFile.iocopy(res.Body, 0, 0, od.config.DiskCache)
	if err != nil {
		od.finish(err)
		return
	}

	od.finish(nil)
}

// singleBreakpoint 单线程，断点续传
func (od *operatDown) singleBreakpoint(ctx context.Context) {
	// 自动处理
	go od.operatFile.operatCF.autoSave(od.config.AutoSaveTnterval)
	// 已分配的数据块
	var (
		err      error
		operatCF = od.operatFile.operatCF
		cf       = operatCF.cf
	)
	for id, block := range cf.threadblock {
		if block.completed == (block.end-block.start)+1 {
			continue
		}
		err = od.singleBreakpointBlock(ctx, id, block.start+block.completed, block.end, block.completed)
		if err != nil {
			od.finish(err)
			return
		}
	}
	// 未分配的任务块
	blockallsize := cf.threadblock[len(cf.threadblock)-1].end + 1
	if od.filesize > blockallsize {
		od.operatFile.operatCF.addTreadblock(0, blockallsize, od.filesize-1)
		id := len(cf.threadblock) - 1
		err = od.singleBreakpointBlock(ctx, id, blockallsize, od.filesize-1, 0)
		if err != nil {
			od.finish(err)
			return
		}
	}
	od.finish(nil)
}

// singleBreakpointBlock 断点续传单数据块
func (od *operatDown) singleBreakpointBlock(ctx context.Context, id int, start, end, completed int64) error {
	res, err := od.rangeDo(ctx, start, end)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// 写入到文件
	err = od.operatFile.iocopy(res.Body, start, id, int(end-start+1))
	if err != nil {
		return err
	}

	return nil
}
