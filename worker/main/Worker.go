package main

import (
	"flag"
	"fmt"
	"lurise/crontab/worker"
	"runtime"
	"time"
)

var (
	confFile string
)

//解析启动参数
func initArgs() {
	flag.StringVar(&confFile, "config", "./worker.json", "the path of config")
	flag.Parse()
}

//初始化线程数量
func initEnv() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {

	var (
		err error
	)
	//获取启动参数
	initArgs()
	//设置环境变量
	initEnv()

	//TODO:对于etcd以及mongodb是否已启动已有校验，但未做日志打印

	if err = worker.InitConfig(confFile); err != nil {
		goto ERR
	}
	if err = worker.InitEtcdMgr(); err != nil {
		goto ERR
	}
	//初始化任务管理器
	if err = worker.InitJobMgr(); err != nil {
		goto ERR
	}
	if err = worker.InitJobExcutor(); err != nil {
		goto ERR
	}

	//启动服务注册协程
	if err = worker.InitRegister(); err != nil {
		goto ERR
	}

	//启动调度器
	if err = worker.InitScheduler(); err != nil {
		goto ERR
	}

	//启动日志协程
	if err = worker.InitLogSink(); err != nil {
		goto ERR
	}

	//初始化drone调度器
	if err = worker.InitDroneScheduler(); err != nil {
		goto ERR
	}
	//初始化drone管理器
	if err = worker.InitDroneMgr(); err != nil {
		goto ERR
	}

	//初始化drone执行器
	if err = worker.InitDroneExcutor(); err != nil {
		goto ERR
	}

	if err = worker.InitDockerMgr(); err != nil {
		goto ERR
	}

	for {
		time.Sleep(1 * time.Second)
	}

	//正常退出
	return
ERR:
	fmt.Println(err)
}
