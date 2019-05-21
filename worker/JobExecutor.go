package worker

import (
	"k8s.io/apimachinery/pkg/util/rand"
	"lurise/crontab/common"
	"os/exec"
	"time"
)

type JobExecutor struct {
}

var (
	G_JobExecutor *JobExecutor
)

func InitJobExcutor() (err error) {
	G_JobExecutor = &JobExecutor{}
	return
}

func (executor *JobExecutor) ExecuteJob(info *common.JobExcuteInfo) {

	go func() {
		var (
			cmd    *exec.Cmd
			output []byte
			err    error
			result *common.JobExcuteResult

			jobLock *Lock
		)

		result = &common.JobExcuteResult{
			ExcuteInfo: info,
			Output:     make([]byte, 0),
		}

		//初始化分布式锁，就是用G_jobMgr的kv，client以及release对象
		jobLock = G_jobMgr.CreateJobLock(info.Job.Name)

		//设置任务的启动时间
		result.StartTime = time.Now()

		//随机睡眠
		time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
		//上锁
		err = jobLock.TryLock(common.JOB_LOCK_DIR + jobLock.name)
		defer jobLock.UnLock()

		//判断是否抢锁成功
		if err != nil {
			result.Err = err
			result.EndTime = time.Now()
		} else {
			result.StartTime = time.Now()
			//运行shell命令

			cmd = exec.CommandContext(info.Ctx, "/bin/bash", "-c", info.Job.Command)
			output, err = cmd.CombinedOutput()

			//构建执行结果
			result.EndTime = time.Now()
			result.Output = output
			result.Err = err
		}

		//回传结果
		G_scheduler.PushJobExcuteResult(result)
	}()
}
