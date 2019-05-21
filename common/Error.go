package common

import "errors"

var (
	ERR_LOCK_ALREADY_REQUIRED = errors.New("锁已被占用")

	ERR_NO_LOCAL_IP_FOUND = errors.New("没有找到IP地址")

	ERR_MASTER_UNALVAILABLE = errors.New("master节点不可用")
)
