package version

import (
	"strings"
)

// FuzzyMatchVersion 使用模糊匹配规则检查两个版本字符串是否匹配
// 它处理类似 "4.32.0" 匹配 "4.32" 的情况，其中尾随的零被忽略。
// 比较通过比较所有公共部分并检查额外部分是否来完成。
//
// 只对第二个参数（API返回的版本）进行清理：去除 -SNAPSHOT 后缀和 .jar 扩展名
// 保留版本号后缀（如 -plat, -gyl 等小组名称）
//
// 匹配规则：
// - 输入 4.34 → 只匹配没有小组的版本（如 4.34.0-SNAPSHOT.jar）
// - 输入 4.34-plat → 只匹配 plat 小组的版本（如 4.34.0-plat-SNAPSHOT.jar）
// - 输入 4.34.0 → 匹配 4.34.0 或 4.34.0-xxx（有无小组都可以）
//
// 示例：
//   - FuzzyMatchVersion("4.34", "4.34.0-SNAPSHOT.jar") == true   (无小组匹配无小组)
//   - FuzzyMatchVersion("4.34", "4.34.0-plat-SNAPSHOT.jar") == false (无小组不匹配有小组)
//   - FuzzyMatchVersion("4.34-plat", "4.34.0-plat-SNAPSHOT.jar") == true (小组匹配小组)
//   - FuzzyMatchVersion("4.34-plat", "4.34.0-gyl-SNAPSHOT.jar") == false (小组不匹配)
func FuzzyMatchVersion(inputVersion, apiVersion string) bool {
	// 只清理 API 返回的版本，保留其后缀（如 -plat）
	cleanApiVersion := cleanVersion(apiVersion)

	// 先尝试精确匹配
	if inputVersion == cleanApiVersion {
		return true
	}

	// 检查用户输入是否包含小组名称
	inputHasGroup := hasGroup(inputVersion)
	apiHasGroup := hasGroup(cleanApiVersion)

	if inputHasGroup {
		// 用户输入包含小组名称，必须与 API 版本的小组名称完全匹配
		group1 := extractGroup(inputVersion)
		group2 := extractGroup(cleanApiVersion)
		if group1 != group2 {
			return false
		}
	} else {
		// 用户输入没有小组名称，API 版本也不能有小组名称
		if apiHasGroup {
			return false
		}
	}

	parts1 := splitVersion(inputVersion)
	parts2 := splitVersion(cleanApiVersion)

	minLen := len(parts1)
	if len(parts2) < minLen {
		minLen = len(parts2)
	}

	// 比较公共部分（检查是否是前缀关系）
	for i := 0; i < minLen; i++ {
		// 检查 parts1[i] 是否是 parts2[i] 的前缀，或相等
		if !isPrefix(parts1[i], parts2[i]) {
			return false
		}
	}

	// 如果用户输入是 API 版本的前缀（包括 -xxx 后缀），允许匹配
	if len(parts1) <= len(parts2) {
		return true
	}

	// API版本比用户输入短时，检查额外部分是否全为0
	return allZeros(parts1[len(parts2):])
}

// cleanVersion 清理版本号，去除 -SNAPSHOT 后缀和 .jar 扩展名
func cleanVersion(version string) string {
	// 先去除 .jar
	version = strings.TrimSuffix(version, ".jar")
	// 去除 -SNAPSHOT 或 -xxx-SNAPSHOT 模式
	if idx := strings.Index(version, "-SNAPSHOT"); idx > 0 {
		version = version[:idx]
	}
	return version
}

// splitVersion 按点分割版本字符串
func splitVersion(version string) []string {
	return strings.Split(version, ".")
}

// allZeros 检查切片中的所有部分是否为 "0"
func allZeros(parts []string) bool {
	for _, part := range parts {
		if part != "0" {
			return false
		}
	}
	return true
}

// isPrefix 检查 s 是否是 prefix 的前缀，或者相等
func isPrefix(s, prefix string) bool {
	return strings.HasPrefix(prefix, s)
}

// hasGroup 检查版本是否包含小组名称（-xxx- 模式）
func hasGroup(version string) bool {
	// 查找 -xxx- 模式，其中 xxx 是字母组成的小组名
	parts := strings.Split(version, ".")
	if len(parts) > 1 {
		// 检查最后一部分是否包含 -xxx 模式
		lastPart := parts[len(parts)-1]
		return strings.Contains(lastPart, "-")
	}
	return false
}

// extractGroup 提取版本中的小组名称
func extractGroup(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) > 1 {
		lastPart := parts[len(parts)-1]
		// 提取 - 之后的部分作为小组名称
		if idx := strings.Index(lastPart, "-"); idx >= 0 {
			return lastPart[idx+1:]
		}
	}
	return ""
}