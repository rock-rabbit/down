package down

// multith 多线程下载
func (operat *operation) multith() {
	if err := operat.operatFile.file.Truncate(operat.size); err != nil {
		operat.finish(err)
		return
	}
	task := threadTaskSplit(0, operat.size, int64(operat.down.ThreadSize))
	// 任务执行
	groupPool := NewWaitGroupPool(operat.down.ThreadCount)
	// 自动保存控制文件
	go operat.operatCF.autoSave(operat.down.AutoSaveTnterval)
	// 每秒给 Hook 发送信息
	go operat.sendStat(groupPool)
	// 执行多线程任务
	go operat.startMultith(groupPool, task)
	// 阻塞等待所有线程完成后返回结果
	var tmperr error
	for err := range groupPool.AllDone() {
		if err != nil {
			tmperr = err
		} else {
			operat.finish(tmperr)
			return
		}
	}
}

// startMultith 执行多线程
func (operat *operation) startMultith(groupPool *WaitGroupPool, task [][2]int64) {
	for idx, fileRange := range task {
		groupPool.Add()
		// 中途关闭
		if operat.contextIsDone() {
			groupPool.Done()
			break
		}
		operat.operatCF.addTreadblock(0, fileRange[0], fileRange[1])
		go operat.multithSingle(idx, groupPool, fileRange[0], fileRange[1], 0)
	}
	// 非阻塞等待所有任务完成
	groupPool.Syne()
}

// multithSingle 多线程下载中单个线程的下载逻辑
func (operat *operation) multithSingle(id int, groupPool *WaitGroupPool, start, end, completed int64) {
	defer groupPool.Done()
	res, err := operat.rangeDo(start, end)
	if err != nil {
		groupPool.Error(err)
		return
	}
	defer res.Body.Close()
	// 写入到文件
	err = operat.operatFile.iocopy(res.Body, start, id, int(end-start+1))
	if err != nil {
		groupPool.Error(err)
		return
	}
}

// multithBreakpoint 多线程，断点续传
func (operat *operation) multithBreakpoint() {
	// 任务执行
	groupPool := NewWaitGroupPool(operat.down.ThreadCount)
	// 自动保存控制文件
	go operat.operatCF.autoSave(operat.down.AutoSaveTnterval)
	// 每秒给 Hook 发送信息
	go operat.sendStat(groupPool)
	// 执行多线程任务
	go operat.startMultithBreakpoint(groupPool)
	// 阻塞等待所有线程完成后返回结果
	var tmperr error
	for err := range groupPool.AllDone() {
		if err != nil {
			tmperr = err
		} else {
			operat.finish(tmperr)
			return
		}
	}
}

// startMultith 执行多线程
func (operat *operation) startMultithBreakpoint(groupPool *WaitGroupPool) {
	// 已分配的数据块
	for id, block := range operat.operatCF.cf.threadblock {
		if block.completed == (block.end-block.start)+1 {
			continue
		}
		groupPool.Add()
		// 中途关闭
		if operat.contextIsDone() {
			groupPool.Done()
			break
		}
		go operat.multithSingle(id, groupPool, block.start+block.completed, block.end, block.completed)
	}
	// 未分配的任务块
	threadblocklen := len(operat.operatCF.cf.threadblock)
	startsize := operat.operatCF.cf.threadblock[threadblocklen-1].end + 1
	for idx, task := range threadTaskSplit(startsize, operat.size, int64(operat.down.ThreadSize)) {
		groupPool.Add()
		// 中途关闭
		if operat.contextIsDone() {
			groupPool.Done()
			break
		}
		operat.operatCF.addTreadblock(0, task[0], task[1])
		id := threadblocklen + idx
		go operat.multithSingle(id, groupPool, task[0], task[1], 0)
	}

	groupPool.Syne()
}
