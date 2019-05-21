package worker

import (
	ContextGo "context"
	"github.com/docker/docker/api/types/volume"
	"time"

	"github.com/docker/distribution/context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	"lurise/crontab/common"
)

//docker控制结构
type DockerMgr struct {
	cli *client.Client

	errch  <-chan error
	status <-chan container.ContainerWaitOKBody
}

var (
	G_DockerMgr *DockerMgr
)

//初始化docker控制
func InitDockerMgr() (err error) {
	var (
		cli *client.Client
		ctx ContextGo.Context
	)
	ctx = ContextGo.Background()
	if cli, err = client.NewClientWithOpts(client.FromEnv); err != nil {
		return
	}
	cli.NegotiateAPIVersion(ctx)

	G_DockerMgr = &DockerMgr{
		cli:    cli,
		errch:  make(<-chan error, 100),
		status: make(<-chan container.ContainerWaitOKBody, 100),
	}
	return
}

//创建容器
func (dockerMgr *DockerMgr) CreateContainer(dockerRunConfig *common.DockerRunConfig) (dockerID string, err error) {
	var (
		containerCreateCreatedBody container.ContainerCreateCreatedBody
	)
	if containerCreateCreatedBody, err = dockerMgr.cli.ContainerCreate(context.Background(),
		dockerRunConfig.ContainerConfig,
		dockerRunConfig.HostConfig,
		dockerRunConfig.NetWorkConfig,
		dockerRunConfig.ContainerName); err != nil {
		return
	}

	dockerID = containerCreateCreatedBody.ID
	return dockerID, nil
}

//启动容器
func (dockerMgr *DockerMgr) StartContainer(containerID string) (err error) {
	var (
		ctx ContextGo.Context
	)

	ctx = ContextGo.Background()

	err = dockerMgr.cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{})

	return
}

//停止容器
func (dockerMgr *DockerMgr) StopContainer(containerID string) (err error) {
	var (
		ctx          ContextGo.Context
		timeDuration time.Duration
	)

	ctx = ContextGo.Background()
	timeDuration = time.Duration(G_Config.ContainerStopTimeout) * time.Millisecond

	err = dockerMgr.cli.ContainerStop(ctx, containerID, &timeDuration)

	return
}

//删除容器
func (dockerMgr *DockerMgr) RemoveContainer(containerID string, containerRemoveOptions types.ContainerRemoveOptions) (err error) {
	var (
		ctx ContextGo.Context
	)

	ctx = ContextGo.Background()
	err = dockerMgr.cli.ContainerRemove(ctx, containerID, containerRemoveOptions)

	return
}

//等待容器状态
func (dockerMgr *DockerMgr) WaitForContainer(containerID string) {
	var (
		ctx ContextGo.Context
		err error
	)
	ctx = ContextGo.Background()

	dockerMgr.status, dockerMgr.errch = dockerMgr.cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	select {
	case err = <-dockerMgr.errch:
		err = err
		//TODO:err未做处理
	case <-dockerMgr.status:
		//TODO：可以记录容器的状态;返回容器的状态
	}

	return
}

//获取镜像列表
func (dockerMgr *DockerMgr) ListImage() (imageTags map[string]string, err error) {
	var (
		images []types.ImageSummary
	)
	imageTags = make(map[string]string, 0)

	if images, err = dockerMgr.cli.ImageList(ContextGo.Background(), types.ImageListOptions{}); err != nil {
		return
	}

	for _, image := range images {
		for _, tag := range image.RepoTags {
			imageTags[tag] = image.ID
		}
	}
	return
}

//根据镜像名判断镜像是否存在
func (dockerMgr *DockerMgr) IsImageExistByTag(repTags string) (isExist bool) {
	var (
		imageTags map[string]string
		err       error
	)

	if imageTags, err = dockerMgr.ListImage(); err != nil {
		return false
	}

	_, isExist = imageTags[common.PackImageName(repTags)]

	return
}

//根据镜像名拉取镜像
func (dockerMgr *DockerMgr) PullImageByTag(repTag string) (err error) {
	_, err = dockerMgr.cli.ImagePull(context.Background(), repTag, types.ImagePullOptions{})
	return
}

//创建volumns
func (dockerMgr *DockerMgr) CreateVolumn(name string, labels map[string]string) (volumn types.Volume, err error) {
	volumn, err = dockerMgr.cli.VolumeCreate(context.Background(), volume.VolumeCreateBody{
		Name:   name,
		Driver: "local",
		Labels: labels,
	})

	return
}
