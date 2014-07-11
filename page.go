package main

type eventPage []*FileEvent

func (p *eventPage) progress() progress {
	prog := make(progress)

	for _, event := range *p {
		if *event.Source == "-" || event.Source == nil || event.Rotated {
			continue
		}

		ino, dev := file_ids(event.fileinfo)
		prog[*event.Source] = &FileState{
			Source: event.Source,
			Offset: event.Offset + int64(len(*event.Text)) + 1,
			Inode:  ino,
			Device: dev,
		}
	}

	return prog
}

func (p *eventPage) counts() (map[string]int, map[string]int) {
	total, bad := make(map[string]int), make(map[string]int)
	for _, event := range *p {
		if event.Source == nil {
			continue
		}
		total[*event.Source] += 1
		if event.Rotated {
			bad[*event.Source] += 1
		}
	}
	return total, bad
}

func (p *eventPage) empty() bool {
	return len(*p) == 0
}
