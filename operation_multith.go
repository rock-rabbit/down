package down

import "context"

// multith 多线程下载
func (od *operatDown) multith(ctx context.Context) {
	if err := od.operatFile.file.Truncate(od.filesize); err != nil {
		od.finish(err)
		return
	}
	task := threadTaskSplit(0, od.filesize, int64(od.config.ThreadSize))
	// 自动处理
	go od.operatFile.operatCF.autoSave(od.config.AutoSaveTnterval)
	// 执行多线程任务
	go od.startMultith(ctx, task)
	// 阻塞等待所有线程完成后返回结果
	var tmperr error
	for err := range od.wgpool.AllDone() {
		if err != nil {
			tmperr = err
		} else {
			od.finish(tmperr)
			return
		}
	}
}

// startMultith 执行多线程
func (od *operatDown) startMultith(ctx context.Context, task [][2]int64) {
	for idx, fileRange := range task {
		od.wgpool.Add()
		if contextDone(ctx) {
			od.wgpool.Done()
			break
		}
		od.operatFile.operatCF.addTreadblock(0, fileRange[0], fileRange[1])
		go od.multithSingle(ctx, idx, fileRange[0], fileRange[1], 0)
	}
	// 非阻塞等待所有任务完成
	od.wgpool.Syne()
}

// multithSingle 多线程下载中单个线程的下载逻辑
func (od *operatDown) multithSingle(ctx context.Context, id int, start, end, completed int64) {
	defer od.wgpool.Done()
	res, err := od.rangeDo(ctx, start, end)
	if err != nil {
		od.wgpool.Error(err)
		return
	}
	defer res.Body.Close()
	// 写入到文件
	err = od.operatFile.iocopy(res.Body, start, id, int(end-start+1))
	if err != nil {
		od.wgpool.Error(err)
		return
	}
}

// multithBreakpoint 多线程，断点续传
func (od *operatDown) multithBreakpoint(ctx context.Context) {
	// 自动处理
	go od.operatFile.operatCF.autoSave(od.config.AutoSaveTnterval)
	// 执行多线程任务
	go od.startMultithBreakpoint(ctx)
	// 阻塞等待所有线程完成后返回结果
	var tmperr error
	for err := range od.wgpool.AllDone() {
		if err != nil {
			tmperr = err
		} else {
			od.finish(tmperr)
			return
		}
	}
}

// startMultith 执行多线程
func (od *operatDown) startMultithBreakpoint(ctx context.Context) {
	var operatCF = od.operatFile.operatCF
	// 已分配的数据块
	for id, block := range operatCF.cf.threadblock {
		if block.completed == (block.end-block.start)+1 {
			continue
		}
		od.wgpool.Add()
		// 中途关闭
		if contextDone(ctx) {
			od.wgpool.Done()
			break
		}
		go od.multithSingle(ctx, id, block.start+block.completed, block.end, block.completed)
	}
	// 未分配的任务块
	threadblocklen := len(operatCF.cf.threadblock)
	startsize := operatCF.cf.threadblock[threadblocklen-1].end + 1
	for idx, task := range threadTaskSplit(startsize, od.filesize, int64(od.config.ThreadSize)) {
		od.wgpool.Add()
		// 中途关闭
		if contextDone(ctx) {
			od.wgpool.Done()
			break
		}
		operatCF.addTreadblock(0, task[0], task[1])
		id := threadblocklen + idx
		go od.multithSingle(ctx, id, task[0], task[1], 0)
	}

	od.wgpool.Syne()
}
