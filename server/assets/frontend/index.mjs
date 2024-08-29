import { Terminal } from './xterm.min.mjs'

var term = new Terminal();
term.open(document.getElementById('terminal'));
term.write('Hello from \x1B[1;3;31mxterm.js\x1B[0m\r\n')

async function main() {
  let search = new URLSearchParams(location.search)
  let agent_name = prompt("agent name?", search.get('agent') || "");
  if (!agent_name) {
    term.write(`\x1B[1;3;31m[${agent_name}]\x1B[0m No agent name\r\n`);
    return
  }

  search.set('agent', agent_name)
  history.replaceState({}, "", `?${search}`)

  const url = `./api/client/${agent_name}/pty/`;
  const ws = new WebSocket(url);

  ws.onopen = () => {
    term.write(`\x1B[1;3;32m[${agent_name}]\x1B[0m Connected\r\n`);

    ws.send(new TextEncoder().encode('\x01sh'));
  };

  ws.onmessage = async (e) => {
    let data = new Uint8Array(await e.data.arrayBuffer())
    switch (data[0]) {
      case 0x00:
        term.write(data.slice(1));
        break;
      case 0x01:
        term.write(`\x1B[1;3;32m[${agent_name}]\x1B[0m Pty Opened\r\n`);
        break;
      case 0x02:
        term.write(`\x1B[1;3;31m[${agent_name}]\x1B[0m Pty Closed, connection intact.\r\n`);
        break;
      case 0x03:
        // term.write(`\x1B[1;3;33m[${agent_name}]\x1B[0m ${data.slice(1)}\r\n`);
        console.log(data.slice(1));
        break;
    }
  };
  ws.onclose = () => {
    term.write(`\x1B[1;3;31m[${agent_name}]\x1B[0m Connection Closed\r\n`);
  };

  term.onData((data) => {
    const bufData = new TextEncoder().encode(data);
    const buf = new Uint8Array([0x00, ...bufData]);

    ws.send(buf);
  });
}

main()