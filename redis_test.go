package redis

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/letsfire/redigo"
	"github.com/letsfire/redigo/mode/alone"
)

var mode = alone.New(
	alone.Addr("192.168.0.110:6379"),
)
var expiry = time.Second
var redriver = New(redigo.New(mode))

func TestRedisDriver_Lock(t *testing.T) {
	var counter int32
	var waitGroup = new(sync.WaitGroup)
	var name, value, other = "test.lock.name", "test.lock.value", "test.lock.value.other"

	/* test for competition of lock in multiple goroutine */
	for i := 0; i < 100; i++ {
		waitGroup.Add(1)
		go func() {
			ok, _ := redriver.Lock(name, value, expiry)
			if ok {
				atomic.AddInt32(&counter, 1)
			}
			waitGroup.Done()
		}()
	}
	waitGroup.Wait()
	if counter != 1 {
		t.Errorf("unexpected result, expect = 1, but = %d", counter)
	}

	/* test for only assign the same value can unlock the mutex */
	redriver.Unlock(name, other)
	ok1, _ := redriver.Lock(name, value, expiry)
	redriver.Unlock(name, value)
	ok2, _ := redriver.Lock(name, value, expiry)
	if ok1 || !ok2 {
		t.Errorf("unexpected result, expect = [false,true] but = [%#v,%#v]", ok1, ok2)
	}

	/* test for the broadcast notification which triggered by unlock */
	commonWatchAndNotifyTest(
		name,
		func() bool {
			ok, _ := redriver.Lock(name, value, expiry)
			return ok
		},
		func() {
			redriver.Unlock(name, value)
		},
	)
}

func TestRedisDriver_Touch(t *testing.T) {
	var name, value, other = "test.touch.name{a}", "test.touch.value", "test.touch.value.other"
	ok1 := redriver.Touch(name, value, expiry)
	redriver.Lock(name, value, expiry)
	ok2 := redriver.Touch(name, value, expiry)
	ok3 := redriver.Touch(name, other, expiry)
	if ok1 || !ok2 || ok3 {
		t.Errorf("unexpected result, expect = [false,true,false] but = [%#v,%#v,%#v]", ok1, ok2, ok3)
	}
}

func TestRedisDriver_RLock(t *testing.T) {
	var counter int32
	var waitGroup = new(sync.WaitGroup)
	var name, value, other = "test.rlock.name", "test.rlock.value", "test.rlock.value.other"

	/* test for read locks are not mutually exclusive */
	for i := 0; i < 10; i++ {
		waitGroup.Add(1)
		go func() {
			ok, _ := redriver.RLock(name, value, expiry)
			if ok {
				atomic.AddInt32(&counter, 1)
			}
			waitGroup.Done()
		}()
	}
	waitGroup.Wait()
	if counter != 10 {
		t.Errorf("unexpected result, expect = 10, but = %d", counter)
	}

	/* test for write is mutually exclusive with read,
	 * no read lock can enter in after write lock if even it is failed */
	ok1, _ := redriver.WLock(name, value, expiry)
	ok2, _ := redriver.RLock(name, value, expiry)
	if ok1 || ok2 {
		t.Errorf("unexpected result, expect = [false,false], but = [%#v,%#v]", ok1, ok2)
	}

	/* test for only assign the same value can unlock the mutex
	 * after read unlock, if write lock tried before, it will be preferential */
	for i := 0; i < 10; i++ {
		redriver.RUnlock(name, other)
	}
	ok3, _ := redriver.WLock(name, value, expiry)
	for i := 0; i < 10; i++ {
		redriver.RUnlock(name, value)
	}
	ok4, _ := redriver.RLock(name, value, expiry)
	ok5, _ := redriver.WLock(name, value, expiry)
	redriver.WUnlock(name, value)
	ok6, _ := redriver.RLock(name, value, expiry)
	if ok3 || ok4 || !ok5 || !ok6 {
		t.Errorf("unexpected result, expect = [false,false,true,true], but = [%#v,%#v,%#v,%#v]", ok3, ok4, ok5, ok6)
	}

	/* test for the broadcast notification which triggered by unlock */
	commonWatchAndNotifyTest(
		name,
		func() bool {
			ok, _ := redriver.RLock(name, value, expiry)
			return ok
		},
		func() {
			redriver.RUnlock(name, value)
		},
	)
}

func TestRedisDriver_WLock(t *testing.T) {
	var counter int32
	var waitGroup = new(sync.WaitGroup)
	var name, value, other = "test.wlock.name", "test.wlock.value", "test.wlock.value.other"

	/* test for write locks are mutually exclusive */
	for i := 0; i < 10; i++ {
		waitGroup.Add(1)
		go func() {
			ok, _ := redriver.WLock(name, value, expiry)
			if ok {
				atomic.AddInt32(&counter, 1)
			}
			waitGroup.Done()
		}()
	}
	waitGroup.Wait()
	if counter != 1 {
		t.Errorf("unexpected result, expect = 1, but = %d", counter)
	}

	/* test for read is mutually exclusive with write,
	 * only assign the same value can unlock the mutex */
	ok1, _ := redriver.RLock(name, value, expiry)
	redriver.WUnlock(name, other)
	ok2, _ := redriver.RLock(name, value, expiry)
	redriver.WUnlock(name, value)
	ok3, _ := redriver.RLock(name, value, expiry)
	if ok1 || ok2 || !ok3 {
		t.Errorf("unexpected result, expect = [false,false,true], but = [%#v,%#v,%#v]", ok1, ok2, ok3)
	}

	/* test for the broadcast notification which triggered by unlock */
	commonWatchAndNotifyTest(
		name,
		func() bool {
			ok, _ := redriver.WLock(name, value, expiry)
			return ok
		},
		func() {
			redriver.WUnlock(name, value)
		},
	)
}

func commonWatchAndNotifyTest(name string, lock func() bool, unlock func()) {
	unlock()
	var waitGroup sync.WaitGroup
	for i := 0; i < 5; i++ {
		waitGroup.Add(1)
		go func() {
			msgCounter := 0
			notifyChan := redriver.Watch(name)
			for {
				select {
				case <-notifyChan:
					msgCounter++
					if msgCounter == 100 {
						waitGroup.Done()
					}
				}
			}
		}()
	}
	waitGroup.Add(1)
	go func() {
		/* make ensure all channels are ready */
		time.Sleep(time.Millisecond * 1200)
		for i := 1; i <= 100; i++ {
			if lock() {
				unlock()
			}
		}
		waitGroup.Done()
	}()
	waitGroup.Wait()
}
