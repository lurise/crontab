package common

const (
	//任务目录
	JOB_SAVE_DIR = "/cron/jobs/"
	//job强杀目录
	JOB_KILLER_DIR = "/cron/killer/"

	//服务注册目录
	WORKER_DIR = "/cron/worker/"
	//master注册目录
	MASTER_DIR = "master/"

	//drone目录
	DRONE_SAVE_DIR = "/drone/jobs/"
	//drone强杀目录
	DRONE_KILLER_DIR = "/drone/killer/"
	//drone在执行目录
	DRONE_WORKING_DIR = "/drone/excute/"

	//job事件的类型
	JOB_EVENT_SAVE   = 1
	JOB_EVENT_DELETE = 2
	JOB_EVENT_KILLER = 3

	//drone事件的类型
	DRONE_EVENT_SAVE   = 1
	DRONE_EVENT_DELETE = 2
	DRONE_EVENT_KILLER = 3

	//任务锁目录
	JOB_LOCK_DIR = "/cron/lock/"

	//Drone锁目录
	DRONE_LOCK_DIR = "drone/lock/"
)
