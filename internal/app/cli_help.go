package app

import (
	"flag"
	"fmt"
	"strings"
)

func setCommandHelp(flags *flag.FlagSet, summary string) {
	flags.Usage = func() {
		fmt.Fprintf(flags.Output(), "用法：%s [参数]\n\n%s\n\n参数：\n", flags.Name(), summary)
		writeFlagHelp(flags)
	}
}

func writeMainHelp() {
	output := flag.CommandLine.Output()
	fmt.Fprintln(output, "用法：")
	fmt.Fprintln(output, "  webhook [参数]                  启动 Webhook 服务")
	fmt.Fprintln(output, "  webhook agent [参数]            临时加入 Hub")
	fmt.Fprintln(output, "  webhook update <子命令> [参数]   检查更新、应用更新或回滚")
	fmt.Fprintln(output, "\n配置文件：")
	fmt.Fprintln(output, "  -c, -config, --config 路径\n    \t读取 YAML 总配置，命令行参数优先于配置文件")
	fmt.Fprintln(output, "\n参数：")
	writeFlagHelp(flag.CommandLine)
	fmt.Fprintln(output, "\n使用 webhook agent --help 或 webhook update --help 查看子命令帮助。")
}

func writeFlagHelp(flags *flag.FlagSet) {
	output := flags.Output()
	flags.VisitAll(func(option *flag.Flag) {
		name, usage := flag.UnquoteUsage(option)
		switch name {
		case "string":
			name = "字符串"
		case "int", "int64":
			name = "整数"
		case "uint", "uint64":
			name = "非负整数"
		case "float":
			name = "数值"
		case "duration":
			name = "时长"
		case "value":
			name = "值"
		}
		fmt.Fprintf(output, "  -%s", option.Name)
		if name != "" {
			fmt.Fprintf(output, " %s", name)
		}
		fmt.Fprintf(output, "\n    \t%s", strings.ReplaceAll(usage, "\n", "\n    \t"))
		// Custom list flags use String as an example value, not a default.
		getter, hasDefault := option.Value.(flag.Getter)
		if hasDefault && option.DefValue != "" && option.DefValue != "0" && option.DefValue != "false" && option.DefValue != "0s" {
			value := option.DefValue
			if _, isString := getter.Get().(string); isString {
				value = fmt.Sprintf("%q", value)
			}
			fmt.Fprintf(output, "（默认值：%s）", value)
		}
		fmt.Fprintln(output)
	})
	fmt.Fprintln(output, "  -h, --help\n    \t显示帮助后退出")
}
