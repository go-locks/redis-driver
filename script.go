package redis

import (
	"github.com/gomodule/redigo/redis"
)

var (
	// KEYS = [name]
	// ARGV = [value, expiry]
	lockScript = redis.NewScript(1, `
		if redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2], "NX") then
			return -3
		end
		return redis.call("PTTL", KEYS[1])
	`)

	// KEYS = [name, channel]
	// ARGV = [value]
	unlockScript = redis.NewScript(2, `
		if (redis.call("GET", KEYS[1]) == ARGV[1]) then
			redis.call("DEL", KEYS[1])
			redis.call("PUBLISH", KEYS[2], 1)
		end
	`)

	// KEYS = [name]
	// ARGV = [value, expiry]
	touchScript = redis.NewScript(1, `
		if (redis.call("GET", KEYS[1]) == ARGV[1]) then
			redis.call("PEXPIRE", KEYS[1], ARGV[2])
			return 1
		end
		return 0
	`)

	// KEYS = [name]
	// ARGV = [value, expiry]
	readLockScript = redis.NewScript(1, `
		local count = tonumber(redis.call("HGET", KEYS[1], "count"))
		if (count ~= nil) and (count < 0) then
			return redis.call("PTTL", KEYS[1])
		end
		redis.call("HINCRBY", KEYS[1], ARGV[1], 1)
		redis.call("HINCRBY", KEYS[1], "count", 1)
		redis.call("PEXPIRE", KEYS[1], ARGV[2])
		return -3
	`)

	// KEYS = [name, channel]
	// ARGV = [value, maxReaders]
	readUnlockScript = redis.NewScript(2, `
		local count = tonumber(redis.call("HGET", KEYS[1], "count"))
		local readMode = ((count ~= nil) and ((count >= 0) or (count + ARGV[2] >= 0)))
		if readMode and (redis.call("HEXISTS", KEYS[1], ARGV[1]) == 1) then
			if (redis.call("HINCRBY", KEYS[1], ARGV[1], -1) == 0) then
				redis.call("HDEL", KEYS[1], ARGV[1])
			end
			local newCount = redis.call("HINCRBY", KEYS[1], "count", -1)
			if (newCount == 0) or (newCount + ARGV[2] == 0) then
				redis.call("PUBLISH", KEYS[2], 1)
			end
		end
	`)

	// KEYS = [name]
	// ARGV = [value, expiry, maxReaders]
	writeLockScript = redis.NewScript(1, `
		local count = tonumber(redis.call("HGET", KEYS[1], "count"))
		if (count == nil) or (count >= 0) then
			redis.call("HINCRBY", KEYS[1], "count", -ARGV[3])
		end
		local canWrite = ((count == nil) or (count == 0) or (count + ARGV[3] == 0))
		if canWrite and (redis.call("HLEN", KEYS[1]) <= 1) then
			redis.call("HSET", KEYS[1], ARGV[1], 1)
			redis.call("PEXPIRE", KEYS[1], ARGV[2])
			return -3
		end
		return redis.call("PTTL", KEYS[1])
	`)

	// KEYS = [name, channel]
	// ARGV = [value, maxReaders]
	writeUnlockScript = redis.NewScript(2, `
		local count = tonumber(redis.call("HGET", KEYS[1], "count"))
		local writeMode = ((count ~= nil) and (count < 0))
		if writeMode and (redis.call("HDEL", KEYS[1], ARGV[1]) == 1) then
			redis.call("HINCRBY", KEYS[1], "count", ARGV[2])
			redis.call("PUBLISH", KEYS[2], 1)
		end
	`)

	// KEYS = [name]
	// ARGV = [value, expiry]
	readWriteTouchScript = redis.NewScript(1, `
		if (redis.call("HEXISTS", KEYS[1], ARGV[1]) == 1) then
			redis.call("PEXPIRE", KEYS[1], ARGV[2])
			return 1
		end
		return 0
	`)
)
