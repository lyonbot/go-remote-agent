package proxy

import (
	"errors"
)

type ProxyChannel struct {
	Disposed bool
}

func NewProxyChannel(agent_name string, agent_id string) (p *ProxyChannel, err error) {
	// tunnel, _, _, notifyAgent, C_to_agent, C_from_agent, err := agent_handler.MakeAgentTunnel(agent_name, agent_id)
	// if err != nil {
	// 	return nil, err
	// }

	return nil, errors.New("not implemented")
}

func (channel *ProxyChannel) Accept() {

}
