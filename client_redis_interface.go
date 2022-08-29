package dlock

type RedisConnInterface interface {
	Close()

	ExecCmd(cmd string, args ...interface{}) (interface{}, error)
	ExecLuaScript(src string, keyCount int, keysAndArgs ...interface{}) (interface{}, error)
}
