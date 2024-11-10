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

type StartPtyRequest struct {
	Cmd        string   `msg:"cmd"`
	Args       []string `msg:"args"`
	Env        []string `msg:"env"`
	InheritEnv bool     `msg:"inherit_env"`
}

type ProxyHttpRequest struct {
	Method  string            `msg:"method"`
	URL     string            `msg:"url"`
	Headers []ProxyHttpHeader `msg:"headers"`
	Body    []byte            `msg:"body"` // beware: disallowed for ws:// or wss://
}

type ProxyHttpResponse struct {
	ConnectionError string            `msg:"connection_error"`
	StatusCode      int32             `msg:"status_code"`
	Headers         []ProxyHttpHeader `msg:"headers"`
}

type ProxyHttpHeader struct {
	Name  string `msg:"name"`
	Value string `msg:"value"`
}
