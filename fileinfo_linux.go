package main

import (
	"fmt"
	"os"
	"syscall"
)

func file_ids(info os.FileInfo) (uint64, uint64) {
	fstat := info.Sys().(*syscall.Stat_t)

	return fstat.Ino, fstat.Dev
}

func filestring(info os.FileInfo) string {
	stat := info.Sys().(*syscall.Stat_t)
	return fmt.Sprintf("%v_%v", stat.Ino, stat.Dev)
}
