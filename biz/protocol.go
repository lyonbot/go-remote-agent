package biz

// see https://github.com/tinylib/msgp/wiki/Getting-Started

//go:generate msgp

type AgentNotify struct {
	Type string `msg:"type"` // shell, ping
	Id   string `msg:"id"`

	// shell
	Cmd        string `msg:"cmd"`
	HasStdin   bool   `msg:"has_stdin"`
	NeedStdout bool   `msg:"need_stdout"`
	NeedStderr bool   `msg:"need_stderr"`
}
