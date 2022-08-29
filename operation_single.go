package down

// single 单线程，非断点续传
func (operat *operation) single() {
	if err := operat.operatFile.file.Truncate(operat.size); err != nil {
		operat.err = err
		operat.finish(err)
		return
	}
	// 自动保存控制文件
	go operat.operatCF.autoSave(operat.down.AutoSaveTnterval)
	// 每秒给 Hook 发送信息
	go operat.sendStat(nil)
	// 执行下载任务
	operat.operatCF.addTreadblock(0, 0, operat.size-1)
	res, err := operat.defaultDo(nil)
	if err != nil {
		operat.err = err
		operat.finish(err)
		return
	}
	defer res.Body.Close()

	// 写入文件
	err = operat.operatFile.iocopy(res.Body, 0, 0, operat.operatFile.bufsize)
	if err != nil {
		operat.err = err
		operat.finish(err)
		return
	}

	operat.finish(nil)
}

// singleBreakpoint 单线程，断点续传
func (operat *operation) singleBreakpoint() {
	// 自动保存控制文件
	go operat.operatCF.autoSave(operat.down.AutoSaveTnterval)
	// 每秒给 Hook 发送信息
	go operat.sendStat(nil)
	// 已分配的数据块
	var err error
	for id, block := range operat.operatCF.cf.threadblock {
		if block.completed == (block.end-block.start)+1 {
			continue
		}
		err = operat.singleBreakpointBlock(id, block.start+block.completed, block.end, block.completed)
		if err != nil {
			operat.err = err
			operat.finish(err)
			return
		}
	}
	// 未分配的任务块
	blockallsize := operat.operatCF.cf.threadblock[len(operat.operatCF.cf.threadblock)-1].end + 1
	if operat.size > blockallsize {
		operat.operatCF.addTreadblock(0, blockallsize, operat.size-1)
		id := len(operat.operatCF.cf.threadblock) - 1
		err = operat.singleBreakpointBlock(id, blockallsize, operat.size-1, 0)
		if err != nil {
			operat.err = err
			operat.finish(err)
			return
		}
	}
	operat.finish(nil)
}

// singleBreakpointBlock 断点续传单数据块
func (operat *operation) singleBreakpointBlock(id int, start, end, completed int64) error {
	res, err := operat.rangeDo(start, end)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// 写入到文件
	err = operat.operatFile.iocopy(res.Body, start, id, int(end-start+1))
	if err != nil {
		return err
	}

	return nil
}
