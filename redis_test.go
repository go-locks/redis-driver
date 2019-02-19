package redis

import (
	"testing"

	"github.com/go-locks/testcase"
	"github.com/letsfire/redigo"
	"github.com/letsfire/redigo/mode/alone"
)

var mode1 = alone.New(
	alone.Addr("192.168.0.110:6379"),
)
var mode2 = alone.New(
	alone.Addr("192.168.0.110:6380"),
)
var mode3 = alone.New(
	alone.Addr("192.168.0.110:6381"),
)
var redriver = New(
	redigo.New(mode1),
	redigo.New(mode2),
	redigo.New(mode3),
)

func TestRedisDriver_Lock(t *testing.T) {
	testcase.RunLockTest(redriver, t)
}

func TestRedisDriver_Touch(t *testing.T) {
	testcase.RunTouchTest(redriver, t)
}

func TestRedisDriver_RLock(t *testing.T) {
	testcase.RunRLockTest(redriver, t)
}

func TestRedisDriver_RTouch(t *testing.T) {
	testcase.RunRTouchTest(redriver, t)
}

func TestRedisDriver_WLock(t *testing.T) {
	testcase.RunWLockTest(redriver, t)
}

func TestRedisDriver_WTouch(t *testing.T) {
	testcase.RunWTouchTest(redriver, t)
}
