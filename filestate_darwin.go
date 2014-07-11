package main

type FileState struct {
	Source string
	Offset int64
	Inode  uint64
	Device int32
}
