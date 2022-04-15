package os

import (
	"BFT/libs/log"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
)

// TrapSignal 捕捉 os.Interrupt 和 syscall.SIGTERM 两个信号，如果捕获到任意一个信号，
// 并且回调函数不为空，则调用回调函数 cb。实际上 os.Interrupt 等于 syscall.SIGINT
func TrapSignal(logger log.CRLogger, cb func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		for sig := range c {
			logger.Infow(fmt.Sprintf("captured %v, exiting...", sig))
			if cb != nil {
				cb() //
			}
			os.Exit(0)
		}
	}()
}

// Kill 让运行中的进程发送 syscall.SIGTERM 信号，
// 该信号会被 TrapSignal 捕获到
func Kill() error {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}
	return p.Signal(syscall.SIGTERM)
}

// Exit 退出程序
func Exit(s string) {
	fmt.Printf(s + "\n")
	os.Exit(1)
}

// EnsureDir 如果给定的目录地址 dir 不存在，则创建目录 dir，
// 但是如果 dir 表示的是一个已存在的非目录路径，则会返回错误
func EnsureDir(dir string, mode os.FileMode) error {
	err := os.MkdirAll(dir, mode)
	if err != nil {
		return fmt.Errorf("could not create directory %q: %w", dir, err)
	}
	return nil
}

// FileExists 判断给定的文件路径是否存在，存在的话返回 true
func FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

// ReadFile 读取整个文件的内容，读取成功的话，返回的错误应该是 nil，不可能返回 EOF
func ReadFile(filePath string) ([]byte, error) {
	return ioutil.ReadFile(filePath)
}

// WriteFile 向指定的文件中写入内容，如果指定的文件不存在，则会直接创建一个文件，然后再写入
func WriteFile(filePath string, contents []byte, mode os.FileMode) error {
	return ioutil.WriteFile(filePath, contents, mode)
}

// MustWriteFile 与 WriteFile 的功能一样，不同的地方在于，如果写内容失败，则会直接退出程序
func MustWriteFile(filePath string, contents []byte, mode os.FileMode) {
	err := WriteFile(filePath, contents, mode)
	if err != nil {
		Exit(fmt.Sprintf("MustWriteFile failed: %v", err))
	}
}

// CopyFile 将源文件里的内容拷贝到目的文件里，如果目的文件已经存在，则把目的文件里的内容截断
func CopyFile(src, dst string) error {
	srcfile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcfile.Close()

	info, err := srcfile.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() {
		return errors.New("cannot read from directories")
	}

	dstfile, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer dstfile.Close()

	_, err = io.Copy(dstfile, srcfile)
	return err
}
