package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

/* 获取指定路径下以及所有子目录下的所有文件，可匹配后缀过滤（suffix为空则不过滤）*/
func WalkDir(dir, suffix string) (files []string, err error) {
	files = []string{}

	err = filepath.Walk(dir, func(fname string, fi os.FileInfo, err error) error {
		if fi.IsDir() {
			//忽略目录
			return nil
		}

		if strings.HasSuffix(strings.ToLower(fi.Name()), suffix) &&
			!strings.HasSuffix(strings.ToLower(fi.Name()), ".txt") &&
			!strings.HasSuffix(strings.ToLower(fi.Name()), ".go"){
			//文件后缀匹配
			files = append(files, fname)
		}

		return nil
	})

	return files, err
}

func ScanLine(files []string) int {
	lines := 0
	for _, f := range files {
		codes, err := os.OpenFile(f, os.O_RDONLY, 0666)
		if err != nil {
			return -1
		}
		buf := bufio.NewReader(codes)
		for {
			str, err := buf.ReadString('\n')
			if err == io.EOF {
				break
			} else if err != nil {
				return -1
			}
			if str != "\n" {
				lines++
			}
			if strings.Contains(str, "聊天室") {
				fmt.Println(f, str)
			}
		}

	}
	return lines
}

func main() {
	//files, err := WalkDir("/root/Experiment/codes/go/src/tendermint", ".go")
	//if err != nil {
	//	log.Fatalln(err)
	//}
	//lineA := ScanLine(files)
	lineA := 1

	files, err := WalkDir("/root/Experiment/codes/go/src/chat-room/", "")
	if err != nil {
		log.Fatalln(err)
	}
	lineB := ScanLine(files) - 70

	progress := float64(lineB) / float64(lineA)

	//f, err := os.OpenFile("build/progress.txt", os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	//if err != nil {
	//	log.Fatalln(err)
	//}

	t := time.Now().Format(time.RFC3339)

	content := fmt.Sprintf("时间：%-25v，目标：%-6d行，已完成：%-6d行，完成度：%.2f%%\n", t, lineA, lineB, progress*100)
	//f.WriteString(content)
	fmt.Println(content)
}
