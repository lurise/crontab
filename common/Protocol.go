package common

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/gorhill/cronexpr"
)

type Job struct {
	Name     string `json:"name"`
	Command  string `json:"command"`
	CronExpr string `json:"cronexpr"`
}

//接口返回信息
type RespMsg struct {
	Errno int         `json:"errno"`
	Msg   string      `json:"msg"`
	Data  interface{} `json:"data"`
}

//任务事件
type JobEvent struct {
	EventType int
	Job       *Job
}

//任务调度计划
type JobSchedulerPlan struct {
	Job      *Job                 //要调度的任务信息
	Expr     *cronexpr.Expression //任务的cron表达式
	NextTime time.Time            //任务下次要执行的时间
}

//任务执行
type JobExcuteInfo struct {
	Job      *Job
	PlanTime time.Time
	RealTime time.Time

	Ctx        context.Context
	CancelFunc context.CancelFunc
}

//任务执行结果
type JobExcuteResult struct {
	ExcuteInfo *JobExcuteInfo
	Output     []byte
	Err        error
	StartTime  time.Time
	EndTime    time.Time
}

//执行任务日志
type JobLog struct {
	JobName      string `bson:"jobName"`
	Command      string `bson:"command"`
	Err          string `bson:"err"`
	Output       string `bson:"output"`
	PlanTime     int64  `bson:"plantime"`
	ScheduleTime int64  `bson:"scheduletime"`
	StartTime    int64  `bson:"starttime"`
	EndTime      int64  `bson:"endtime"`
}

//日志批次
type LogBatch struct {
	Logs []interface{}
}

//mongo日志的筛选条件
type LogFilter struct {
	JobName string `bson:"jobName"`
}

//mongo日志的排序条件
type SortLogByStartTime struct {
	SortOrder int `bson:"starttime"`
}

//容器运行配置
type DockerRunConfig struct {
	ContainerConfig *container.Config
	HostConfig      *container.HostConfig
	NetWorkConfig   *network.NetworkingConfig

	ContainerName string
}

type Drone struct {
	Name string `json:"name"`

	GitServerAdrr string `json:"git_server_addr"`
	GitType       string `json:"git_type"`
	GitID         string `json:"git_id"`
	GitSecret     string `json:"git_secret"`

	ServerAddr string `json:"server_addr"`
}

//{name:dronetest,git_server_addr:https://gitlab.com,git_id:ca787b3613764b9ce3408d0c4146d605edc0eb732cd3421bbe918a6ba1f8acaf,git_secret:46763e8a6ed0bf489768986f73dddb3476e9da1c26feab82c4064eaf09b7197e,server_addr:17b795eb.ngrok.io,git_type:}

//构建drone推送事件
type DroneEvent struct {
	EventType int
	Drone     *Drone
}

type DroneSchedulerPlan struct {
	DroneTable map[string]*Drone
}

//任务执行
type DroneExcuteInfo struct {
	Drone     *Drone
	StartTime time.Time

	Ctx        context.Context
	CancelFunc context.CancelFunc
}

//任务执行结果
type DroneExcuteResult struct {
	ExcuteInfo *DroneExcuteInfo
	Output     []byte    `json:"output"`
	Err        error     `json:"err"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`

	DroneIP string `json:"drone_ip"`
	Port    string `json:"port"`

	ExcuteSuceed bool `json:"excute_succeed"`
}

func PackDroneExcuteResult(result DroneExcuteResult) (resultJson string, err error) {
	var (
		resp []byte
	)
	if resp, err = json.Marshal(result); err != nil {
		return
	}

	resultJson = string(resp)
	return
}
func BuildRespMsg(errno int, msg string, data interface{}) (respByte []byte, err error) {
	var (
		respMsg RespMsg
	)
	respMsg = RespMsg{
		Errno: errno,
		Msg:   msg,
		Data:  data,
	}

	respByte, err = json.Marshal(respMsg)

	return
}

//处理镜像名称，如果镜像名称没有带着标签，则增加latest标签
func PackImageName(imageName string) (newImageName string) {
	if strings.Contains(imageName, ":") {
		newImageName = imageName
	} else {
		newImageName = imageName + ":latest"
	}
	return
}

func UnpackJob(value []byte) (ret *Job, err error) {
	var (
		job *Job
	)
	job = &Job{}
	if err = json.Unmarshal(value, job); err != nil {
		return
	}
	ret = job
	return
}

func UnpackDrone(value []byte) (ret *Drone, err error) {
	var (
		drone *Drone
	)
	test := string(value)
	test = test
	drone = &Drone{}
	if err = json.Unmarshal(value, drone); err != nil {
		return
	}
	ret = drone
	return
}

func ExtractDroneName(droneKey string) string {
	return strings.Trim(droneKey, DRONE_SAVE_DIR)
}

//从etcd的key中提取任务名
// /cron/jobs/job10中提取job10
func ExtractJobName(jobKey string) string {
	return strings.Trim(jobKey, JOB_SAVE_DIR)
}

//从etcd的key中提取任务名
// 从/cron/killer/job10中提取job10
func ExtractKillerName(jobKey string, TrimString string) string {
	return strings.Trim(jobKey, TrimString)
}

//从etcd的key中提取worker地址
func ExtractWorkerAddr(workerKey string) string {
	return strings.Trim(workerKey, WORKER_DIR)
}

//构建Job推送事件
func BuildJobEvent(eventType int, job *Job) (event *JobEvent) {
	return &JobEvent{
		EventType: eventType,
		Job:       job,
	}
}

//构建Drone推送事件
func BuildDroneEvent(eventType int, drone *Drone) (event *DroneEvent) {
	return &DroneEvent{
		EventType: eventType,
		Drone:     drone,
	}
}

//构建jobPlan
func BuildJobPlan(job *Job) (jobSchedulerPlan *JobSchedulerPlan, err error) {
	var (
		expr *cronexpr.Expression
	)

	if expr, err = cronexpr.Parse(job.CronExpr); err != nil {
		return
	}

	jobSchedulerPlan = &JobSchedulerPlan{
		Job:      job,
		Expr:     expr,
		NextTime: expr.Next(time.Now()),
	}
	return
}

//构建执行任务
func BuildJobExcute(jobSchedulePlan *JobSchedulerPlan) (jobExcuteInfo *JobExcuteInfo) {
	var (
		ctx        context.Context
		cancelFunc context.CancelFunc
	)
	ctx, cancelFunc = context.WithCancel(context.TODO())
	jobExcuteInfo = &JobExcuteInfo{
		Job:        jobSchedulePlan.Job,
		PlanTime:   jobSchedulePlan.NextTime,
		RealTime:   time.Now(),
		Ctx:        ctx,
		CancelFunc: cancelFunc,
	}
	return
}

//构建drone任务
func BuildDroneExcute(droneName string, droneSchedulePlan *DroneSchedulerPlan) (droneExcuteInfo *DroneExcuteInfo) {
	var (
		ctx        context.Context
		cancelFunc context.CancelFunc
	)
	ctx, cancelFunc = context.WithCancel(context.TODO())
	droneExcuteInfo = &DroneExcuteInfo{
		Drone:      droneSchedulePlan.DroneTable[droneName],
		StartTime:  time.Now(),
		Ctx:        ctx,
		CancelFunc: cancelFunc,
	}
	return
}

//构建容器运行配置
func BuildDockerRunConfig(containerConfig *container.Config,
	hostConfig *container.HostConfig,
	networkConfig *network.NetworkingConfig,
	containerName string) (dockerRunConfig *DockerRunConfig) {
	dockerRunConfig = &DockerRunConfig{
		ContainerConfig: containerConfig,
		HostConfig:      hostConfig,
		NetWorkConfig:   networkConfig,
		ContainerName:   containerName,
	}

	return
}

//获取本机IP地址
func GetLocalIP() (ipv4 string, err error) {
	var (
		addrs   []net.Addr
		addr    net.Addr
		ipNet   *net.IPNet
		isIpNet bool
	)

	if addrs, err = net.InterfaceAddrs(); err != nil {
		return
	}

	//去第一个非io的网卡IP
	for _, addr = range addrs {
		if ipNet, isIpNet = addr.(*net.IPNet); isIpNet && !ipNet.IP.IsLoopback() {
			//跳过ipv6
			if ipNet.IP.To4() != nil {
				ipv4 = ipNet.IP.String()
				return
			}
		}
	}

	err = ERR_NO_LOCAL_IP_FOUND
	return
}
