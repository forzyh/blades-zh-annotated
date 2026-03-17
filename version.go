package blades

import "runtime/debug"

var (
	// Version 是当前 blades 库的版本号。
	// 版本号在编译时从构建信息中自动读取。
	// 如果使用了 replace 指令，会返回 replace 目标的版本。
	Version = buildVersion("github.com/go-kratos/blades")
)

// buildVersion 从构建信息中检索指定模块路径的版本号。
// 此函数使用 runtime/debug.ReadBuildInfo() 读取依赖版本。
// 这对于诊断和日志记录很有用，可以知道运行时使用的库版本。
//
// 参数：
//   - path: 模块路径，如 "github.com/go-kratos/blades"
//
// 返回：
//   - string: 版本号（如 "v0.1.0"），如果未找到则返回空字符串
func buildVersion(path string) string {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, d := range buildInfo.Deps {
		if d.Path == path {
			// 如果有 replace 指令，返回 replace 目标的版本
			if d.Replace != nil {
				return d.Replace.Version
			}
			return d.Version
		}
	}
	return ""
}
