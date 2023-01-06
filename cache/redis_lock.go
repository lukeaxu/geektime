package cache

import (
	"context"
	_ "embed"
	"errors"
	"github.com/go-redis/redis/v9"
	"github.com/google/uuid"
	"time"
)

var (
	ErrFailedToPreemptLock = errors.New("redis-lock: 抢锁失败")
	ErrLockNotHold         = errors.New("redis-lock: 你没有持有锁")

	//go:embed lua/unlock.lua
	luaUnlock string
)

// Client 就是对 redis.Cmdable 的二次封装
type Client struct {
	client redis.Cmdable
}

func NewClient(client redis.Cmdable) *Client {
	return &Client{
		client: client,
	}
}

func (c *Client) TryLock(ctx context.Context,
	key string,
	expiration time.Duration) (*Lock, error) {
	val := uuid.New().String()
	ok, err := c.client.SetNX(ctx, key, val, expiration).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		// 代表的是别人抢到了锁
		return nil, ErrFailedToPreemptLock
	}
	return &Lock{
		client: c.client,
		key: key,
		value: val,
	}, nil
}

//func (c *Client) Unlock(ctx context.Context, key string) error {
//
//}

/*func (c *Client) Unlock(ctx context.Context, lock *Lock) error {

}*/



type Lock struct {
	client redis.Cmdable
	key string
	value string
}

func (l *Lock) Unlock(ctx context.Context) error {
	res, err := l.client.Eval(ctx, luaUnlock, []string{l.key}, l.value).Int64()
	//if err == redis.Nil {
	//	return ErrLockNotHold
	//}
	if err != nil {
		return err
	}
	if res != 1 {
		return ErrLockNotHold
	}
	return nil
}

//func (l *Lock) Unlock(ctx context.Context) error {
//	// 我现在要先判断一下，这把锁是不是我的锁
//
//	val, err := l.client.Get(ctx, l.key).Result()
//	if err != nil {
//		return err
//	}
//	if val != l.value {
//		return errors.New("锁不是自己的锁")
//	}
//
//	// 在这个地方，键值对被人删了，紧接着另外一个实例加锁
//
//	// 把键值对删掉
//	cnt, err := l.client.Del(ctx, l.key).Result()
//	if err != nil {
//		return err
//	}
//	if cnt != 1 {
//		// 这个地方代表什么？
//		// 代表你加的锁，过期了
//		//log.Info("redis-lock: 解锁失败，锁不存在")
//		//return nil
//		return ErrLockNotHold
//	}
//	return nil
//}