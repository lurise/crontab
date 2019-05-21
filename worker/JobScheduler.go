package worker

import (
	"fmt"
	"lurise/crontab/common"
	"time"
)

type JobScheduler struct {
	jobEventChan      chan *common.JobEvent
	jobResultChan     chan *common.JobExcuteResult
	jobPlanTable      map[string]*common.JobSchedulerPlan //任务计划
	jobExecutingTable map[string]*common.JobExcuteInfo    //任务执行
}

var (
	G_scheduler *JobScheduler
)

//协程初始化
func InitScheduler() (err error) {
	//初始化全局G_scheduler
	G_scheduler = &JobScheduler{
		jobPlanTable:      make(map[string]*common.JobSchedulerPlan),
		jobExecutingTable: make(map[string]*common.JobExcuteInfo),

		jobResultChan: make(chan *common.JobExcuteResult, 1000),
		jobEventChan:  make(chan *common.JobEvent, 1000),
	}

	//启动调度协程
	go G_scheduler.schedulerLoop()

	return
}

//调度协程
func (scheduler *JobScheduler) schedulerLoop() {
	var (
		jobEvent        *common.JobEvent
		schedulerAfter  time.Duration
		schedulerTimer  *time.Timer
		jobExcuteResult *common.JobExcuteResult
	)

	//初始化一次（1秒）;在初始化时，暂时还没有从etcd中获取任务列表，所以这里应该是1秒
	schedulerAfter = scheduler.TrySchedule()

	//调度的延时定时器
	schedulerTimer = time.NewTimer(schedulerAfter)

	for {
		//堵塞for循环，只有当如下事件发生时会继续进行：
		//1. 有任务的put，delete以及kill动作
		//2. 任务到期，休眠结束
		//3. 有任务执行完成
		select {
		case jobEvent = <-scheduler.jobEventChan: //监听任务变化事件
			scheduler.handleJobEvent(jobEvent)
		case <-schedulerTimer.C: //最近的任务到期了，这里的schedulerTimer的定时时长是最近任务的时间
		case jobExcuteResult = <-scheduler.jobResultChan: //监听任务执行结果
			scheduler.handleJobExcuteResult(jobExcuteResult)
		}
		//调度一次任务
		schedulerAfter = scheduler.TrySchedule()
		//重置调度器：有可能是因为监听到事件变化才到这一步，也有可能是执行了一个任务，需要设置延迟到下次的休眠时长
		schedulerTimer.Reset(schedulerAfter)
	}

}

//处理任务事件
func (scheduler *JobScheduler) handleJobEvent(jobEvent *common.JobEvent) {
	var (
		jobSchedulerPlan *common.JobSchedulerPlan
		err              error
		jobPlanExisted   bool

		jobExecuteInfo *common.JobExcuteInfo
		jobExecuting   bool
	)
	switch jobEvent.EventType {
	//保存任务事件
	case common.JOB_EVENT_SAVE:
		//将事件插入到jobSchedulerPlan中
		if jobSchedulerPlan, err = common.BuildJobPlan(jobEvent.Job); err != nil {
			return
		}
		//将任务插入任务执行列表
		scheduler.jobPlanTable[jobEvent.Job.Name] = jobSchedulerPlan

	//删除任务事件
	case common.JOB_EVENT_DELETE:
		//先判断事件是否存在，如果存在则删除，如果不存在则不做任何处理
		if jobSchedulerPlan, jobPlanExisted = scheduler.jobPlanTable[jobEvent.Job.Name]; jobPlanExisted {
			delete(scheduler.jobPlanTable, jobEvent.Job.Name)
		}
	//强杀事件
	case common.JOB_EVENT_KILLER:
		if jobExecuteInfo, jobExecuting = scheduler.jobExecutingTable[jobEvent.Job.Name]; jobExecuting {
			jobExecuteInfo.CancelFunc()
		}
	}

}

//处理结果事件
func (scheduler *JobScheduler) handleJobExcuteResult(result *common.JobExcuteResult) {
	var (
		joblog *common.JobLog
	)

	delete(scheduler.jobExecutingTable, result.ExcuteInfo.Job.Name)

	//如果错误不是因为抢锁失败引起的
	if result.Err != common.ERR_LOCK_ALREADY_REQUIRED {
		joblog = &common.JobLog{
			JobName:      result.ExcuteInfo.Job.Name,
			Command:      result.ExcuteInfo.Job.Command,
			Output:       string(result.Output),
			PlanTime:     result.ExcuteInfo.PlanTime.UnixNano() / 1000 / 1000,
			ScheduleTime: result.ExcuteInfo.RealTime.UnixNano() / 1000 / 1000,
			StartTime:    result.StartTime.UnixNano() / 1000 / 1000,
			EndTime:      result.EndTime.UnixNano() / 1000 / 1000,
		}

		if result.Err != nil {
			joblog.Err = result.Err.Error()
		} else {
			joblog.Err = ""
		}

		G_logSink.Append(joblog)
	}

	fmt.Println("任务执行完成：", result.ExcuteInfo.Job.Name, string(result.Output), result.Err)
}

//尝试执行任务
func (scheduler *JobScheduler) TryStartJob(jobPlan *common.JobSchedulerPlan) {
	var (
		jobExcuteInfo *common.JobExcuteInfo
		jobExcuting   bool
	)

	//判断任务是否在执行，如果在执行则跳过本次执行
	if jobExcuteInfo, jobExcuting = scheduler.jobExecutingTable[jobPlan.Job.Name]; jobExcuting {
		return
	}

	//构建执行状态信息
	jobExcuteInfo = common.BuildJobExcute(jobPlan)

	//将执行状态放入执行状态表中
	scheduler.jobExecutingTable[jobPlan.Job.Name] = jobExcuteInfo

	//执行job
	G_JobExecutor.ExecuteJob(jobExcuteInfo)
}

//重新计算任务调度状态
func (scheduler *JobScheduler) TrySchedule() (schedulerAfter time.Duration) {
	var (
		jobplan  *common.JobSchedulerPlan
		now      time.Time
		nearTime *time.Time
	)
	//如果任务表为空，则随便睡眠多久
	if len(scheduler.jobPlanTable) == 0 {
		schedulerAfter = 1 * time.Second
		return
	}

	now = time.Now()
	for _, jobplan = range scheduler.jobPlanTable {
		if jobplan.NextTime.Before(now) || jobplan.NextTime.Equal(now) {
			scheduler.TryStartJob(jobplan)
			//fmt.Println("执行任务:", jobplan.Job.Name, " 当前时间是：", time.Now())
			//不管是否执行与否，都需要计算任务下次的执行时间
			jobplan.NextTime = jobplan.Expr.Next(now)
		}

		//统计最近一个要过期的任务，方便主循环进行睡眠
		if nearTime == nil || jobplan.NextTime.Before(*nearTime) {
			nearTime = &jobplan.NextTime
		}
	}
	//下次调度间隔
	schedulerAfter = (*nearTime).Sub(now)
	return
}

//推送任务变化事件
func (scheduler *JobScheduler) PushJobEvent(event *common.JobEvent) {
	scheduler.jobEventChan <- event
}

//推送任务执行结果
func (scheduler *JobScheduler) PushJobExcuteResult(excuteResult *common.JobExcuteResult) {
	scheduler.jobResultChan <- excuteResult
}
