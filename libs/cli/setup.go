package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	HomeFlag     = "home"
	TraceFlag    = "trace"
)

// Executable is the minimal interface to *corba.Command, so we can
// wrap if desired before the test
type Executable interface {
	Execute() error
}

// PrepareBaseCmd is meant for SRBFT and other servers
// PersistentPreRunE: PersistentPreRun 被执行并返回一个错误，*Run函数的执行顺序如下:
//	* PersistentPreRun()
//  * PreRun()
//  * Run()
//  * PostRun()
//  * PersistentPostRun()
// 所有函数都得到相同的参数，即命令名后面的参数。
//	PersistentPreRun:该命令的子命令将继承并执行。
func PrepareBaseCmd(cmd *cobra.Command, envPrefix, defaultHome string) Executor {
	cobra.OnInitialize(func() { initEnv(envPrefix) })
	cmd.PersistentFlags().StringP(HomeFlag, "", defaultHome, "directory for config and data")
	cmd.PersistentFlags().Bool(TraceFlag, false, "print out full stack trace on errors")
	cmd.PersistentPreRunE = concatCobraCmdFuncs(bindFlagsLoadViper, cmd.PersistentPreRunE)
	return Executor{cmd, os.Exit}
}

// initEnv sets to use ENV variables if set.
func initEnv(prefix string) {
	copyEnvVars(prefix)

	// env variables with TM prefix (eg. TM_ROOT)
	viper.SetEnvPrefix(prefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()
}

// 用来重新设置环境变量的格式，例如如果 prefix 是 “SR”，加入环境变量中存在 “SRROOT=.SRBFT”，
// 则 “SRROOT=.SRBFT” 会被改为 “SR_ROOT=.SRBFT”
func copyEnvVars(prefix string) {
	prefix = strings.ToUpper(prefix)
	ps := prefix + "_"
	for _, e := range os.Environ() {
		kv := strings.SplitN(e, "=", 2)
		if len(kv) == 2 {
			k, v := kv[0], kv[1]
			if strings.HasPrefix(k, prefix) && !strings.HasPrefix(k, ps) {
				k2 := strings.Replace(k, prefix, ps, 1)
				os.Setenv(k2, v)
			}
		}
	}
}

// Executor wraps the cobra Command with a nicer Execute method
type Executor struct {
	*cobra.Command
	Exit func(int) // this is os.Exit by default, override in tests
}

type ExitCoder interface {
	ExitCode() int
}

// execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func (e Executor) Execute() error {
	e.SilenceUsage = true
	e.SilenceErrors = true
	err := e.Command.Execute()
	if err != nil {
		if viper.GetBool(TraceFlag) {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			fmt.Fprintf(os.Stderr, "ERROR: %v\n%s\n", err, buf)
		} else {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		}

		// return error code 1 by default, can override it with a special error type
		exitCode := 1
		if ec, ok := err.(ExitCoder); ok {
			exitCode = ec.ExitCode()
		}
		e.Exit(exitCode)
	}
	return err
}

type cobraCmdFunc func(cmd *cobra.Command, args []string) error

// Returns a single function that calls each argument function in sequence
// RunE, PreRunE, PersistentPreRunE, etc. all have this same signature
func concatCobraCmdFuncs(fs ...cobraCmdFunc) cobraCmdFunc {
	return func(cmd *cobra.Command, args []string) error {
		for _, f := range fs {
			if f != nil {
				if err := f(cmd, args); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

// 绑定所有标志并将配置信息读入 viper
// 搜索并读取配置文件
// 如果有多个名称为 config 的配置文件，viper怎么搜索呢？它会按照如下顺序搜索
//	config.json
//	config.toml
//	config.yaml
//	config.yml
//	config.properties (这种一般是java中的配置文件名)
//	config.props (这种一般是java中的配置文件名)
func bindFlagsLoadViper(cmd *cobra.Command, args []string) error {
	// cmd.Flags() 包括来自此命令的标志和来自父命令的所有持久标志
	// BindPFlags 将一个完整的标志设置绑定到配置中，使用每个标志的长名称作为配置键。
	if err := viper.BindPFlags(cmd.Flags()); err != nil {
		return err
	}

	homeDir := viper.GetString(HomeFlag)
	viper.Set(HomeFlag, homeDir)
	// SetConfigName 设置配置文件的名称，不包括扩展
	viper.SetConfigName("config")                         // 设置准备读取的配置文件的名字，此处不带配置文件的扩展名
	viper.AddConfigPath(homeDir)                          // 添加查找配置文件所在的搜索路径
	viper.AddConfigPath(filepath.Join(homeDir, "config")) // 多次调用 AddConfigPath，可以添加多个搜索路径

	if err := viper.ReadInConfig(); err == nil {

	} else if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
		return err
	}
	return nil
}
