package biz

// see https://github.com/tinylib/msgp/wiki/Getting-Started

//go:generate msgp

type AgentNotify struct {
	Type string `msg:"type"` // ping, shell, pty, upgrade
	Id   string `msg:"id"`

	// shell
	Cmd        string `msg:"cmd"`
	HasStdin   bool   `msg:"has_stdin"`
	NeedStdout bool   `msg:"need_stdout"`
	NeedStderr bool   `msg:"need_stderr"`
}

type FileInfo struct {
	Path  string `msg:"path"`
	Size  int64  `msg:"size"`
	Mode  uint32 `msg:"mode"`
	Mtime int64  `msg:"mtime"`
}
