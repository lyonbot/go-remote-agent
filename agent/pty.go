package agent

import (
	"bytes"
	"encoding/binary"
	"os"
	"os/exec"
	"remote-agent/biz"
	"remote-agent/utils"
	"strings"
	"sync"

	ptylib "github.com/creack/pty"
	"github.com/gorilla/websocket"
)

func run_pty(task *biz.AgentNotify) {
	url := biz.Config.BaseUrl + "/api/agent/" + biz.Config.Name + "/" + task.Id
	url = strings.Replace(url, "http", "ws", 1)
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return
	}
	defer c.Close()

	wg := sync.WaitGroup{}
	ws := utils.WSConnToChannels(c, &wg)
	defer func() {
		utils.TryClose(ws.Write)
		ws.Write = nil // for unclosed pty
		wg.Wait()
	}()

	write_debug_message := func(msg string) {
		ws.Write <- utils.PrependBytes([]byte{0x03}, []byte(msg))
	}

	var pty *os.File
	defer func() {
		if pty != nil {
			pty.Close()
			pty = nil
		}
	}()

	for recv := range ws.Read {
		t := recv[0]

		switch t {
		case 0x00:
			if pty != nil {
				pty.Write(recv[1:])
			}

		case 0x01:
			if pty != nil {
				write_debug_message("pty already opened")
			} else {
				cmd_str := strings.Split(string(recv[1:]), "\x00")
				c := exec.Command(cmd_str[0], cmd_str[1:]...)
				c.Env = append(c.Env, "TERM=xterm-256color")
				new_pty, err := ptylib.Start(c)
				if err != nil {
					write_debug_message(err.Error())
				} else {
					pty = new_pty

					wg.Add(1)
					go func() {
						defer wg.Done()
						defer func() {
							pty.Close()
							pty = nil
							utils.TryWrite(ws.Write, []byte{0x02}) // pty closed
						}()

						for {
							data := make([]byte, 1024)
							n, err := pty.Read(data)
							if err != nil {
								write_debug_message(err.Error())
								return
							}

							ws.Write <- utils.PrependBytes([]byte{0x00}, data[:n])
						}
					}()
					ws.Write <- []byte{0x01} // pty opened
				}
			}

		case 0x02:
			if pty != nil {
				if err := pty.Close(); err != nil {
					write_debug_message(err.Error())
				}
			}

		case 0x04: // upload file chunk
			offset := int64(binary.LittleEndian.Uint64(recv[1:]))
			length := int64(binary.LittleEndian.Uint64(recv[9:]))
			data_since := int64(len(recv)) - length
			path := string(recv[17:data_since])
			data := recv[data_since:]
			if err := write_file_chunk(path, offset, data); err != nil {
				write_debug_message(err.Error())
				break
			}
			ws.Write <- bytes.Join([][]byte{
				[]byte{0x04},
				binary.LittleEndian.AppendUint64(nil, uint64(offset)),
				[]byte(path),
			}, []byte{})

		case 0x05: // read file info
			path := string(recv[1:])
			if info, err := os.Stat(path); err == nil {
				msg := biz.FileInfo{
					Path:  path,
					Size:  int64(info.Size()),
					Mode:  uint32(info.Mode()),
					Mtime: info.ModTime().Unix(),
				}
				msg_bytes, err := msg.MarshalMsg(nil)
				if err != nil {
					write_debug_message(err.Error())
					break
				}
				ws.Write <- utils.PrependBytes([]byte{0x05}, msg_bytes)
			} else {
				write_debug_message(err.Error())
			}

		case 0x06: // read file chunk
			offset := int64(binary.LittleEndian.Uint64(recv[1:]))
			length := int64(binary.LittleEndian.Uint64(recv[9:]))
			file_path := string(recv[17:])

			data, err := read_file_chunk(file_path, offset, length)
			if data == nil {
				write_debug_message(err.Error())
				break
			}

			actual_length := int64(len(data))
			ws.Write <- bytes.Join([][]byte{
				[]byte{0x06},
				binary.LittleEndian.AppendUint64(nil, uint64(offset)),
				binary.LittleEndian.AppendUint64(nil, uint64(actual_length)),
				[]byte(file_path),
				data,
			}, []byte{})
		}
	}
}

// create or update a file.
// if file exists, extend file size and overwrite data to the certain offset
// if data length is 0, it means truncate the file to length of offset
func write_file_chunk(path string, offset int64, data []byte) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	if len(data) == 0 {
		// truncate file
		err = file.Truncate(offset)
		return err
	}

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	// extend file size if necessary
	file_size := stat.Size()
	min_required_size := offset + int64(len(data))
	if min_required_size > file_size {
		// file is not big enough
		if err := file.Truncate(min_required_size); err != nil {
			return err
		}
	}

	// write data
	if _, err := file.WriteAt(data, offset); err != nil {
		return err
	}

	return nil
}

// read a file chunk
func read_file_chunk(path string, offset int64, max_length int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	buf := make([]byte, max_length)
	n, err := file.ReadAt(buf, offset)
	return buf[:n], err
}
