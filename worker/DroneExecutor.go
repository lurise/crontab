package worker

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"lurise/crontab/common"
	"math/rand"
	"time"
)

type DroneExecutor struct {
}

var (
	G_DroneExecutor *DroneExecutor
)

func InitDroneExcutor() (err error) {
	G_DroneExecutor = &DroneExecutor{}
	return
}

func (executor *DroneExecutor) ExecuteDrone(info *common.DroneExcuteInfo) {

	go func() {
		var (
			err  error
			lock *Lock
			port string

			dockerRunConfig *common.DockerRunConfig
			dockerId        string

			result *common.DroneExcuteResult
		)
		result = &common.DroneExcuteResult{
			ExcuteInfo: info,
			Output:     make([]byte, 0),
		}

		//初始化分布式锁，就是用G_DroneMgr的kv，client以及release对象
		lock = G_DroneMgr.CreateDroneLock(info.Drone.Name)

		//设置任务的启动时间
		result.StartTime = time.Now()

		//随机睡眠,平衡worker节点的时间差
		time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
		//上锁
		err = lock.TryLock(common.DRONE_LOCK_DIR + lock.name)
		defer lock.UnLock()

		//判断是否抢锁成功
		if err != nil {
			result.Err = err
			result.EndTime = time.Now()
			result.ExcuteSuceed = false
		} else {
			//1. 判断镜像是否存在，如果不存在则拉取镜像或从本地load
			if !G_DockerMgr.IsImageExistByTag(G_Config.DroneImageTag) {
				if err = G_DockerMgr.PullImageByTag(G_Config.DroneImagePullAddr); err != nil {
					result.Err = err
					result.EndTime = time.Now()
					result.ExcuteSuceed = false
				}
			}

			//2. 配置容器启动参数

			env, err := InitEnv(info)
			if err != nil {
				result.Err = err
				result.EndTime = time.Now()
				result.ExcuteSuceed = false
			}

			for _, port = range G_Config.DroneRunningPorts {
				exports, err, portMap := InitPortBanding(port)
				if err != nil {
					result.Err = err
					result.EndTime = time.Now()
					result.ExcuteSuceed = false

					continue
				}
				//_, err := G_DockerMgr.CreateVolumn(info.Drone.Name, map[string]string{
				//	"/var/lib/docker.sock":           "/var/lib/docker.sock",
				//	"/data/drone/" + info.Drone.Name: "/data",
				//})

				//if err != nil {
				//
				//}
				containerConfig := BuildDroneContainerConfig(env, exports)

				dockerRunConfig = &common.DockerRunConfig{
					ContainerConfig: containerConfig,
					HostConfig: &container.HostConfig{
						Binds: []string{
							"/var/lib/docker.sock:/var/lib/docker.sock",
							"/data/drone/" + info.Drone.Name + ":/data",
						},
						PortBindings: portMap,
					},
					NetWorkConfig: nil,
					ContainerName: info.Drone.Name,
				}
				//3. 启动容器，获取容器启动状态，返回结果
				if dockerId, err = G_DockerMgr.CreateContainer(dockerRunConfig); err != nil {
					result.Err = err
					result.EndTime = time.Now()
					result.ExcuteSuceed = false

					continue
				}
				if err = G_DockerMgr.StartContainer(dockerId); err != nil {
					result.Err = err
					result.EndTime = time.Now()
					result.ExcuteSuceed = false
					G_DockerMgr.RemoveContainer(dockerId, types.ContainerRemoveOptions{})
					continue
				}

				statusCh, errCh := G_DockerMgr.cli.ContainerWait(context.Background(), dockerId, container.WaitConditionNotRunning)
				select {
				case err := <-errCh:
					if err != nil {
						result.Err = err
						result.EndTime = time.Now()
						result.ExcuteSuceed = false

						continue
					}
				case <-statusCh:
					result.ExcuteSuceed = true
					result.DroneIP = G_Register.localIP
					result.Port = port
					break
				}
			}

		}

		G_DroneScheduler.PushDroneResult(result)
		//回传结果
		//G_scheduler.PushJobExcuteResult(result)
	}()
}

func BuildDroneContainerConfig(env []string, exports nat.PortSet) *container.Config {
	containerConfig := &container.Config{
		Image:        G_Config.DroneImageTag,
		Env:          env,
		ExposedPorts: exports,
		//Volumes:map[string]struct{}{volume.Mountpoint:{}},  //TODO:容器的volumn绑定有点问题，还得测试下
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
		OpenStdin:    false,
		StdinOnce:    false,
		ArgsEscaped:  false,
	}
	return containerConfig
}

//配置drone容器的端口映射
func InitPortBanding(port string) (nat.PortSet, error, nat.PortMap) {

	exports := make(nat.PortSet, 10)
	port1, err := nat.NewPort("tcp", "80")
	if err != nil {
		return nil, err, nil
	}
	//port2, err := nat.NewPort("tcp", "443")
	//if err != nil {
	//	//TODO:处理错误
	//}
	exports[port1] = struct{}{}
	//exports[port2] = struct{}{}
	portBind1 := nat.PortBinding{HostPort: port}
	//portBind2 := nat.PortBinding{HostPort: "443"}
	portMap := make(nat.PortMap, 10)
	tmp1 := make([]nat.PortBinding, 0, 1)
	tmp1 = append(tmp1, portBind1)
	//tmp2 := make([]nat.PortBinding, 0, 1)
	//tmp2 = append(tmp1, portBind2)
	portMap[port1] = tmp1
	//portMap[port2] = tmp2
	return exports, err, portMap
}

//配置drone容器的环境变量
func InitEnv(info *common.DroneExcuteInfo) (env []string, err error) {
	//TODO:没有做gittype的处理
	if G_Register.masterAvailable {
		env = make([]string, 0)
		env = append(env, "DRONE_GIT_ALWAYS_AUTH=false")
		env = append(env, "DRONE_GITLAB_SERVER="+info.Drone.GitServerAdrr)
		env = append(env, "DRONE_GITLAB_CLIENT_ID="+info.Drone.GitID)
		env = append(env, "DRONE_GITLAB_CLIENT_SECRET="+info.Drone.GitSecret)
		env = append(env, "DRONE_RUNNER_CAPACITY=2")
		env = append(env, "DRONE_SERVER_HOST="+G_Register.masterIP)
		env = append(env, "DRONE_SERVER_PROTO=http")

		err = nil
	} else {
		err = common.ERR_MASTER_UNALVAILABLE
	}

	return
}
