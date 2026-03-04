package version

import (
	"strings"
)

// FuzzyMatchVersion 使用模糊匹配规则检查两个版本字符串是否匹配
// 它处理类似 "4.32.0" 匹配 "4.32" 的情况，其中尾随的零被忽略。
// 比较通过比较所有公共部分并检查额外部分是否为零来完成。
//
// 示例：
//   - FuzzyMatchVersion("4.32.0", "4.32") == true
//   - FuzzyMatchVersion("4.32", "4.32.0") == true
//   - FuzzyMatchVersion("4.32.1", "4.32") == false
//   - FuzzyMatchVersion("4.32", "4.32.1") == false
func FuzzyMatchVersion(version1, version2 string) bool {
	parts1 := splitVersion(version1)
	parts2 := splitVersion(version2)

	minLen := len(parts1)
	if len(parts2) < minLen {
		minLen = len(parts2)
	}

	// 比较公共部分
	for i := 0; i < minLen; i++ {
		if parts1[i] != parts2[i] {
			return false
		}
	}

	// 检查较长版本的额外部分是否全为零
	if len(parts1) > len(parts2) {
		return allZeros(parts1[len(parts2):])
	}
	return allZeros(parts2[len(parts1):])
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
