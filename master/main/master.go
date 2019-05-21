package main

import (
	"flag"
	"fmt"
	"lurise/crontab/master"
	"runtime"
	"time"
)

var (
	confFile string
)

//解析启动参数
func initArgs() {
	flag.StringVar(&confFile, "config", "./master.json", "the path of MasterConfig")
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
	//获取启动参数，赋值config的目录
	initArgs()

	//加载配置参数
	if err = master.InitConfig(confFile); err != nil {
		goto ERR
	}
	//设置环境变量
	initEnv()

	//要非常注意启动顺序，调研一下注入程序	drone1

	//启动服务端
	if err = master.InitApiServer(); err != nil {
		goto ERR
	}

	//启动任务管理器
	if err = master.InitJobMgr(); err != nil {
		goto ERR
	}

	//注册master
	if err = master.InitRegister(); err != nil {
		goto ERR
	}

	//启动节点查询服务
	if err = master.InitWorkerMgr(); err != nil {
		goto ERR
	}
	//启动日志服务
	if err = master.InitLogMgr(); err != nil {
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
