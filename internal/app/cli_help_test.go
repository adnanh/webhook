package app

import (
	"os/exec"
	"strings"
	"testing"
)

func TestChineseCommandHelp(t *testing.T) {
	binary, cleanup := buildWebhook(t)
	defer cleanup()
	for _, test := range []struct {
		command string
		want    []string
	}{
		{"", []string{"配置文件：", "-config", "Webhook 服务监听的端口", "默认值：9000"}},
		{"agent", []string{"Hub 基础地址", "自定义反向代理路径前缀", "保存节点令牌", "默认值：1h0m0s"}},
		{"update", []string{"检查可用更新", "下载并应用更新", "恢复上一版本"}},
		{"update check", []string{"发布版本号", "JSON 格式", "默认值："}},
		{"update apply", []string{"交互确认", "可执行文件路径", "更新状态目录"}},
		{"update rollback", []string{"恢复上一版本", "可执行文件路径", "更新状态目录"}},
	} {
		for _, help := range []string{"-h", "--help"} {
			t.Run(test.command+"/"+help, func(t *testing.T) {
				args := append(strings.Fields(test.command), help)
				output, err := exec.Command(binary, args...).CombinedOutput()
				if err != nil {
					t.Fatalf("help failed: %v\n%s", err, output)
				}
				text := string(output)
				for _, want := range append([]string{"用法："}, test.want...) {
					if !strings.Contains(text, want) {
						t.Errorf("help missing %q:\n%s", want, text)
					}
				}
				for _, unwanted := range []string{"Usage:", "Usage of", "(default ", " string\n", " duration\n"} {
					if strings.Contains(text, unwanted) {
						t.Errorf("help contains untranslated text %q", unwanted)
					}
				}
			})
		}
	}
}
