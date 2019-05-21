package worker

import (
	"context"
	"lurise/crontab/common"
	"time"
)

type DroneScheduler struct {
	droneEventChan        chan *common.DroneEvent
	droneExcuteResultChan chan *common.DroneExcuteResult

	dronePlanTable     map[string]*common.Drone
	droneExcutingTable map[string]*common.Drone
}

var (
	G_DroneScheduler *DroneScheduler
)

//协程初始化
func InitDroneScheduler() (err error) {
	//初始化全局G_scheduler
	G_DroneScheduler = &DroneScheduler{
		droneEventChan:        make(chan *common.DroneEvent, 1000),
		droneExcuteResultChan: make(chan *common.DroneExcuteResult, 1000),
		dronePlanTable:        make(map[string]*common.Drone, 1000),
		droneExcutingTable:    make(map[string]*common.Drone, 1000),
	}

	//启动调度协程
	go G_DroneScheduler.DroneSchedulerLoop()

	return
}

//调度协程
func (scheduler *DroneScheduler) DroneSchedulerLoop() {

	var (
		droneEvent        *common.DroneEvent
		droneExcuteResult *common.DroneExcuteResult

		timer *time.Timer
	)
	timer = time.NewTimer(1 * time.Second)
	//TODO:未获取已存储在etcd中的drone配置信息
	//TODO:未获取已在workder节点中运行的节点信息
	for {
		//堵塞for循环，只有当如下事件发生时会继续进行：
		//1. 有drone的put，delete以及kill动作
		//2. 时间超过1秒
		//3. //TODO:对于down掉的worker节点，接收其在运行的drone
		select {
		case droneEvent = <-scheduler.droneEventChan: //监听任务变化事件
			scheduler.handleDroneEvent(droneEvent)
		//case <-timer.C:
		case droneExcuteResult = <-scheduler.droneExcuteResultChan:
			scheduler.handleDroneResult(droneExcuteResult)
		}
		//调度一次任务
		scheduler.TryDroneSchedule()
		timer.Reset(1 * time.Second)
	}
}

//调度drone容器运行
func (scheduler *DroneScheduler) TryDroneSchedule() {
	var (
		droneName   string
		isExecuting bool
	)
	for droneName, _ = range scheduler.dronePlanTable {
		if _, isExecuting = scheduler.droneExcutingTable[droneName]; !isExecuting {
			scheduler.droneExcutingTable[droneName] = scheduler.dronePlanTable[droneName]
			scheduler.TryStartDrone(droneName, scheduler.dronePlanTable[droneName])
		}
	}
	for droneName, _ = range scheduler.droneExcutingTable {
		if _, isExecuting = scheduler.dronePlanTable[droneName]; !isExecuting {
			scheduler.TryStopDrone(droneName, scheduler.droneExcutingTable[droneName])
		}
	}
	return
}

//推送drone变化事件
func (scheduler *DroneScheduler) PushDroneEvent(event *common.DroneEvent) {
	scheduler.droneEventChan <- event
}

//推送drone运行结果
func (scheduler *DroneScheduler) PushDroneResult(result *common.DroneExcuteResult) {
	scheduler.droneExcuteResultChan <- result
}

//TODO:对于down机的节点，该如何进行重启？可能需要监控workder节点的状态，一旦发现网略不通，则需要将该workder节点的所有drone服务转移出来

//处理drone推送事件，启动或删除容器
func (scheduler *DroneScheduler) handleDroneEvent(droneEvent *common.DroneEvent) {
	switch droneEvent.EventType {
	case common.DRONE_EVENT_SAVE:
		scheduler.dronePlanTable[droneEvent.Drone.Name] = droneEvent.Drone
	case common.DRONE_EVENT_DELETE:
		if _, isExist := scheduler.dronePlanTable[droneEvent.Drone.Name]; isExist {
			delete(scheduler.dronePlanTable, droneEvent.Drone.Name)
		}
	case common.JOB_EVENT_KILLER:
		//TODO:获取drone强杀的事件推送
	}
}

//尝试启动drone
func (scheduler *DroneScheduler) TryStartDrone(droneName string, drone *common.Drone) {
	var (
		droneExcuteInfo *common.DroneExcuteInfo
		ctx             context.Context
		cancelFun       context.CancelFunc
	)
	ctx, cancelFun = context.WithCancel(context.TODO())

	droneExcuteInfo = &common.DroneExcuteInfo{
		Drone:      drone,
		StartTime:  time.Now(),
		Ctx:        ctx,
		CancelFunc: cancelFun,
	}

	G_DroneExecutor.ExecuteDrone(droneExcuteInfo)
}

//TODO:处理Drone结果
func (scheduler *DroneScheduler) handleDroneResult(result *common.DroneExcuteResult) {
	var (
		resultJson string
		err        error
	)

	if result.ExcuteSuceed {
		if resultJson, err = common.PackDroneExcuteResult(*result); err != nil {
			return
		}
		//TODO:执行成功则将workder节点的ip地址，运行的端口存储到etcd中 "/drone/excute/drone名"  "{excuteIP:XXX,excutePort:xxx}"
		if _, err = G_DroneMgr.kv.Put(context.TODO(), common.DRONE_WORKING_DIR+result.ExcuteInfo.Drone.Name, resultJson); err != nil {
			return
		}
	} else {
		delete(G_scheduler.jobExecutingTable, result.ExcuteInfo.Drone.Name)
	}
}

func (scheduler *DroneScheduler) TryStopDrone(droneName string, drone *common.Drone) {
	//TODO:尝试停止已删除的drone容器
	//1. 检查本节点是否有该名字的容器
	//2. 停止并删除容器
}
