package worker

import (
	"context"
	"github.com/coreos/etcd/clientv3"
	"lurise/crontab/common"
)

// 分布式锁
type Lock struct {
	//etcd客户端
	kv    clientv3.KV
	lease clientv3.Lease

	name      string             //任务名
	cancelFun context.CancelFunc //取消自动续租
	leaseId   clientv3.LeaseID

	isLocked bool
}

func InitLock(jobName string, kv clientv3.KV, lease clientv3.Lease) (lock *Lock) {
	lock = &Lock{
		kv:    kv,
		lease: lease,
		name:  jobName,
	}

	return
}

func (lock *Lock) TryLock(lockKey string) (err error) {
	var (
		leaseGrantResp         *clientv3.LeaseGrantResponse
		leaseId                clientv3.LeaseID
		leaseKeepAliveRespChan <-chan *clientv3.LeaseKeepAliveResponse

		ctx        context.Context
		cancelFunc context.CancelFunc

		txn     clientv3.Txn
		txnResp *clientv3.TxnResponse
	)

	//1 创建租约:方便节点down掉后，自动释放
	if leaseGrantResp, err = lock.lease.Grant(context.TODO(), 5); err != nil {
		goto FAIL
	}

	//2. context用于取消自动续租
	ctx, cancelFunc = context.WithCancel(context.TODO())

	//获取租约ID
	leaseId = leaseGrantResp.ID

	//3. 自动续租，使用含cancelfunc的context
	if leaseKeepAliveRespChan, err = lock.lease.KeepAlive(ctx, leaseId); err != nil {
		goto FAIL
	}

	//4. 处理续租应答的协程
	go func() {
		var (
			keepResp *clientv3.LeaseKeepAliveResponse
		)

		for {
			select {
			case keepResp = <-leaseKeepAliveRespChan: //自动续租的应答
				//判断释放已取消续租
				if keepResp == nil {
					goto END
				}
			}
		}
	END:
	}()

	//5. 创建事务Txn
	txn = lock.kv.Txn(context.TODO())

	////锁路径
	//lockKey = common.JOB_LOCK_DIR + lock.jobName

	//6. 事务抢锁：判断是否已创建，如未创建则创建锁（即抢占），如已创建则查询锁（无意义）
	txn.If(clientv3.Compare(clientv3.CreateRevision(lockKey), "=", 0)).
		Then(clientv3.OpPut(lockKey, "", clientv3.WithLease(leaseId))).
		Else(clientv3.OpGet(lockKey))

	//7. 提交事务
	if txnResp, err = txn.Commit(); err != nil {
		goto FAIL
	}

	//8.抢锁成功则继续进行，失败则释放租约
	if !txnResp.Succeeded {
		err = common.ERR_LOCK_ALREADY_REQUIRED
		goto FAIL
	}

	//9. 抢锁成功
	lock.leaseId = leaseId
	lock.cancelFun = cancelFunc
	lock.isLocked = true

	return
FAIL:
	cancelFunc()                               //取消自动续租
	lock.lease.Revoke(context.TODO(), leaseId) //主动释放租约
	return
}

func (lock *Lock) UnLock() {
	if lock.isLocked {
		lock.cancelFun()
		lock.lease.Revoke(context.TODO(), lock.leaseId)
	}
}
