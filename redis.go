package redis

import (
	"time"

	"github.com/go-locks/distlock/driver"
	"github.com/gomodule/redigo/redis"
	"github.com/letsfire/redigo"
	"github.com/letsfire/redisc"
	"github.com/sirupsen/logrus"
)

var (
	MaxReaders            = 1 << 30
	MinWatchRetryInterval = time.Millisecond
	MaxWatchRetryInterval = time.Second * 16
)

type redisDriver struct {
	quorum  int
	cluster bool
	redigo  []*redigo.Redigo
}

var _ driver.IWatcher = &redisDriver{}
var _ driver.IDriver = &redisDriver{}
var _ driver.IRWDriver = &redisDriver{}

func New(redigo ...*redigo.Redigo) *redisDriver {
	for _, rg := range redigo {
		rg.Exec(func(c redis.Conn) (res interface{}, err error) {
			lockScript.Load(c)
			unlockScript.Load(c)
			touchScript.Load(c)
			readLockScript.Load(c)
			readUnlockScript.Load(c)
			writeLockScript.Load(c)
			writeUnlockScript.Load(c)
			readWriteTouchScript.Load(c)
			return
		})
	}
	return &redisDriver{
		redigo:  redigo,
		quorum:  (len(redigo) / 2) + 1,
		cluster: redigo[0].Mode() == "cluster",
	}
}

func (rd *redisDriver) channelName(name string) string {
	return "unlock-notify-channel-{" + name + "}"
}

func (rd *redisDriver) doLock(fn func(rg *redigo.Redigo) int) (bool, time.Duration) {
	counter := rd.quorum
	for _, rg := range rd.redigo {
		for {
			if wait := fn(rg); wait == 0 {
				continue
			} else if wait == -3 {
				counter -= 1
				if counter == 0 {
					return true, 0
				}
			} else if wait > 0 {
				return false, time.Duration(wait) * time.Millisecond
			}
			break
		}
	}
	return false, -1
}

func (rd *redisDriver) doTouch(fn func(rg *redigo.Redigo) bool) bool {
	counter := rd.quorum
	for _, rg := range rd.redigo {
		if fn(rg) {
			counter -= 1
			if counter == 0 {
				return true
			}
		}
	}
	return false
}

func (rd *redisDriver) Lock(name, value string, expiry time.Duration) (bool, time.Duration) {
	msExpiry := int(expiry / time.Millisecond)
	return rd.doLock(func(rg *redigo.Redigo) int {
		wait, err := rg.Int(func(c redis.Conn) (res interface{}, err error) {
			if rd.cluster {
				redisc.BindConn(c, name)
			}
			return lockScript.Do(c, name, value, msExpiry)
		})
		if err != nil || wait == -1 {
			wait = -1 // less than zero, use the default wait duration
			logrus.WithError(err).Errorf("redis acquire lock '%s' failed", name)
		}
		return wait
	})
}

func (rd *redisDriver) Unlock(name, value string) {
	channel := rd.channelName(name)
	for _, rg := range rd.redigo {
		_, err := rg.Exec(func(c redis.Conn) (res interface{}, err error) {
			if rd.cluster {
				redisc.BindConn(c, name, channel)
			}
			return unlockScript.Do(c, name, channel, value)
		})
		if err != nil {
			logrus.WithError(err).Errorf("redis release lock '%s' failed", name)
		}
	}
}

func (rd *redisDriver) Touch(name, value string, expiry time.Duration) bool {
	msExpiry := int(expiry / time.Millisecond)
	return rd.doTouch(func(rg *redigo.Redigo) bool {
		ok, err := rg.Bool(func(c redis.Conn) (res interface{}, err error) {
			if rd.cluster {
				redisc.BindConn(c, name)
			}
			return touchScript.Do(c, name, value, msExpiry)
		})
		if err != nil {
			logrus.WithError(err).Errorf("redis touch lock '%s' failed", name)
		}
		return ok
	})
}

func (rd *redisDriver) RLock(name, value string, expiry time.Duration) (bool, time.Duration) {
	msExpiry := int(expiry / time.Millisecond)
	return rd.doLock(func(rg *redigo.Redigo) int {
		wait, err := rg.Int(func(c redis.Conn) (res interface{}, err error) {
			if rd.cluster {
				redisc.BindConn(c, name)
			}
			return readLockScript.Do(c, name, value, msExpiry)
		})
		if err != nil || wait == -1 {
			wait = -1 // less than zero, use the default wait duration
			logrus.WithError(err).Errorf("redis acquire read lock '%s' failed", name)
		}
		return wait
	})
}

func (rd *redisDriver) RUnlock(name, value string) {
	channel := rd.channelName(name)
	for _, rg := range rd.redigo {
		_, err := rg.Exec(func(c redis.Conn) (res interface{}, err error) {
			if rd.cluster {
				redisc.BindConn(c, name, channel)
			}
			return readUnlockScript.Do(c, name, channel, value, MaxReaders)
		})
		if err != nil {
			logrus.WithError(err).Errorf("redis release read lock '%s' failed", name)
		}
	}
}

func (rd *redisDriver) RTouch(name, value string, expiry time.Duration) bool {
	msExpiry := int(expiry / time.Millisecond)
	return rd.doTouch(func(rg *redigo.Redigo) bool {
		ok, err := rg.Bool(func(c redis.Conn) (res interface{}, err error) {
			if rd.cluster {
				redisc.BindConn(c, name)
			}
			return readWriteTouchScript.Do(c, name, value, msExpiry)
		})
		if err != nil {
			logrus.WithError(err).Errorf("redis touch lock '%s' failed", name)
		}
		return ok
	})
}

func (rd *redisDriver) WLock(name, value string, expiry time.Duration) (bool, time.Duration) {
	msExpiry := int(expiry / time.Millisecond)
	return rd.doLock(func(rg *redigo.Redigo) int {
		wait, err := rg.Int(func(c redis.Conn) (res interface{}, err error) {
			if rd.cluster {
				redisc.BindConn(c, name)
			}
			return writeLockScript.Do(c, name, value, msExpiry, MaxReaders)
		})
		if err != nil || wait == -1 {
			wait = -1 // less than zero, use the default wait duration
			logrus.WithError(err).Errorf("redis acquire write lock '%s' failed", name)
		}
		return wait
	})
}

func (rd *redisDriver) WUnlock(name, value string) {
	channel := rd.channelName(name)
	for _, rg := range rd.redigo {
		_, err := rg.Exec(func(c redis.Conn) (res interface{}, err error) {
			if rd.cluster {
				redisc.BindConn(c, name, channel)
			}
			return writeUnlockScript.Do(c, name, channel, value, MaxReaders)
		})
		if err != nil {
			logrus.WithError(err).Errorf("redis release write lock '%s' failed", name)
		}
	}
}

func (rd *redisDriver) WTouch(name, value string, expiry time.Duration) bool {
	return rd.RTouch(name, value, expiry)
}

func (rd *redisDriver) Watch(name string) <-chan struct{} {
	channel := rd.channelName(name)
	outChan := make(chan struct{})
	for _, rg := range rd.redigo {
		go func(rg *redigo.Redigo) {
			errSleepDuration := MinWatchRetryInterval
			for {
				err := rg.Sub(func(c redis.PubSubConn) error {
					if rd.cluster {
						err := redisc.BindConn(c.Conn, channel)
						if err != nil {
							return err
						}
					}
					if err := c.Subscribe(channel); err != nil {
						return err
					}
					errSleepDuration = MinWatchRetryInterval
					for {
						switch v := c.ReceiveWithTimeout(0).(type) {
						case redis.Message:
							outChan <- struct{}{}
						case error:
							return v
						}
					}
				})
				if err != nil {
					logrus.WithError(err).Errorf("redis watch channel '%s' abort", channel)
					time.Sleep(errSleepDuration) // may be the redis server is down
					if errSleepDuration *= 2; errSleepDuration > MaxWatchRetryInterval {
						errSleepDuration = MaxWatchRetryInterval
					}
				}
			}
		}(rg)
	}
	return outChan
}
