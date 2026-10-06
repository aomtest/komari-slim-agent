//go:build windows

package selfupdate

import "errors"

// restartSelf 在 Windows 上不可用。
//
// Windows 不允许替换正在运行的 exe(文件被锁定),所以「下载新版本然后
// rename 覆盖自己」这一步根本无法完成 —— 不是重启方式的问题,而是
// 替换本身做不到。要支持得引入一个独立的更新器进程,复杂度远超收益。
//
// 这里保留一个返回错误的实现(而不是把整个包排除掉),是为了让
// Update() 在 Windows 上给出一条明确的说明,而不是某个费解的 rename 失败。
func restartSelf(exe string) error {
	return errors.New("self-update is not supported on Windows")
}
