package worker

import (
	"context"
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"lurise/crontab/common"
	"strings"
)

//全局单例变量
var G_DroneMgr *DroneMgr

type DroneMgr struct {
	client  *clientv3.Client
	kv      clientv3.KV
	lease   clientv3.Lease
	watcher clientv3.Watcher
}

//定义Etcd的客户端
func InitDroneMgr() (err error) {
	G_DroneMgr = &DroneMgr{
		client:  G_EtcdMgr.Client,
		kv:      G_EtcdMgr.Kv,
		lease:   G_EtcdMgr.Lease,
		watcher: G_EtcdMgr.Watcher,
	}
	//启动监听
	G_DroneMgr.watchDrones()
	G_DroneMgr.watchKiller()
	return
}

//监听/drone/jobs
func (droneMgr *DroneMgr) watchDrones() (err error) {
	var (
		getResp   *clientv3.GetResponse
		keyPair   *mvccpb.KeyValue
		Drone     *common.Drone
		DroneName string

		watchChan          clientv3.WatchChan
		watchResp          clientv3.WatchResponse
		watchEvent         *clientv3.Event
		watchStartRevision int64

		DroneEvent *common.DroneEvent
	)
	//获取现有的Drones
	if getResp, err = droneMgr.kv.Get(context.TODO(), common.DRONE_SAVE_DIR, clientv3.WithPrefix()); err != nil {
		return
	}

	//将现存Drones同步给scheduler
	for _, keyPair = range getResp.Kvs {
		Drone = &common.Drone{}
		if Drone, err = common.UnpackDrone(keyPair.Value); err == nil {
			DroneEvent = common.BuildDroneEvent(common.DRONE_EVENT_SAVE, Drone)
			G_DroneScheduler.PushDroneEvent(DroneEvent)
		}
	}

	watchStartRevision = getResp.Header.Revision + 1
	//启动监听协程
	go func() {
		watchChan = droneMgr.watcher.Watch(context.TODO(), common.DRONE_SAVE_DIR, clientv3.WithRev(watchStartRevision), clientv3.WithPrefix())

		//遍历监听channel
		for watchResp = range watchChan {
			for _, watchEvent = range watchResp.Events {
				switch watchEvent.Type {
				case mvccpb.PUT:
					if Drone, err = common.UnpackDrone(watchEvent.Kv.Value); err != nil {
						continue
					}
					//构建一个添加Event
					DroneEvent = common.BuildDroneEvent(common.DRONE_EVENT_SAVE, Drone)
				case mvccpb.DELETE:
					// Delete /cron/Drones/Drone10
					DroneName = common.ExtractDroneName(string(watchEvent.Kv.Key))
					Drone = &common.Drone{
						Name: DroneName,
					}
					//构造一个删除Event
					DroneEvent = common.BuildDroneEvent(common.DRONE_EVENT_DELETE, Drone)
				}
				//将事件推送给Scheduler
				G_DroneScheduler.PushDroneEvent(DroneEvent)
			}
		}
	}()

	return
}

// 监听/cron/killer
func (droneMgr *DroneMgr) watchKiller() (err error) {
	var (
		Drone *common.Drone

		watchChan  clientv3.WatchChan
		watchResp  clientv3.WatchResponse
		watchEvent *clientv3.Event

		DroneName string

		DroneEvent *common.DroneEvent
	)
	//启动监听协程
	go func() {
		// 监听 /cron/killer/目录的变化
		watchChan = droneMgr.watcher.Watch(context.TODO(), common.DRONE_KILLER_DIR, clientv3.WithPrefix())

		for watchResp = range watchChan {
			for _, watchEvent = range watchResp.Events {
				switch watchEvent.Type {
				case mvccpb.PUT:
					DroneName = strings.Trim(string(watchEvent.Kv.Value), common.DRONE_SAVE_DIR)
					Drone = &common.Drone{
						Name: DroneName,
					}
					//构建一个添加Event
					DroneEvent = common.BuildDroneEvent(common.DRONE_EVENT_KILLER, Drone)
					//将事件推送给Scheduler
					G_DroneScheduler.PushDroneEvent(DroneEvent)
				case mvccpb.DELETE: //killer标记过期，自动删除
				}

			}
		}
	}()

	return
}

//创建drone执行锁，分布式乐观锁
func (droneMgr *DroneMgr) CreateDroneLock(DroneName string) (DroneLock *Lock) {
	DroneLock = InitLock(DroneName, droneMgr.kv, droneMgr.lease)
	return
}
