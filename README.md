[go-locks/distlock](https://github.com/go-locks/distlock) 的 `Redis` 驱动。客户端使用 [letsfire/redigo](https://github.com/letsfire/redigo) 实现，同时支持 `alone` `cluster` `sentinel` 3种部署模式。本驱动支持互斥锁 `mutex` 和读写锁 `rwmutex`。更多使用案例详见 [examples](https://github.com/go-locks/examples)


## 代码实例

Alone模式

```go
var mode = alone.New(
	alone.Addr("192.168.0.110:6379"),
)
var redriver = New(redigo.New(mode))
```

Sentinel模式

```go
var mode = sentinel.New(
	sentinel.Addrs([]string{"192.168.0.110:26379"}),
)
var redriver = New(redigo.New(mode))
```

Cluster模式

```go
var mode = cluster.New(
	cluster.Nodes([]string{
        "192.168.0.110:30001", "192.168.0.110:30002", "192.168.0.110:30003",
        "192.168.0.110:30004", "192.168.0.110:30005", "192.168.0.110:30006",
    }),
)
var redriver = New(redigo.New(mode))
```

所有模式都支持指定多个实例，在过半实例上获取成功才算加锁成功，解决单点故障问题，如下示例

```go
// 至少在2个节点上加锁成功才算获取到锁
var mode1 = alone.New(
	alone.Addr("192.168.0.110:6379"),
)
var mode2 = alone.New(
	alone.Addr("192.168.0.110:6379"),
)
var mode3 = alone.New(
	alone.Addr("192.168.0.110:6379"),
)
var redriver = New(redigo.New(mode1), redigo.New(mode2), redigo.New(mode3))
```