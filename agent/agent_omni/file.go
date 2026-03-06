package agent_omni

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"remote-agent/biz"
	"remote-agent/utils"

	"github.com/tinylib/msgp/msgp"
)

func (s *PtySession) SetupFileTransfer() {
	// upload file chunk
	s.Handlers[0x10] = func(recv []byte) {
		offset := int64(binary.LittleEndian.Uint64(recv[1:]))
		length := int64(binary.LittleEndian.Uint64(recv[9:]))
		data_since := int64(len(recv)) - length
		path := string(recv[17:data_since])
		data := recv[data_since:]
		if err := write_file_chunk(path, offset, data); err != nil {
			s.WriteDebugMessage(err.Error())
			return
		}
		s.Write(utils.JoinBytes2(
			0x10,
			binary.LittleEndian.AppendUint64(nil, uint64(offset)),
			[]byte(path),
		))
	}

	// read file info
	s.Handlers[0x11] = func(recv []byte) {
		path := string(recv[1:])
		info, err := os.Stat(path)
		if err != nil {
			s.WriteDebugMessage(err.Error())
			return
		}

		msg := biz.FileInfo{
			Path:  path,
			Size:  int64(info.Size()),
			Mode:  uint32(info.Mode()),
			Mtime: info.ModTime().Unix(),
		}
		msg_bytes, err := msg.MarshalMsg(nil)
		if err != nil {
			s.WriteDebugMessage(err.Error())
			return
		}
		s.Write(utils.PrependBytes([]byte{0x11}, msg_bytes))
	}

	// list directory
	// response: [0x13] + uint16LE(pathLen) + path + msgpack([]FileInfo)
	s.Handlers[0x13] = func(recv []byte) {
		path := string(recv[1:])
		entries, err := os.ReadDir(path)
		if err != nil {
			s.WriteDebugMessage(err.Error())
			return
		}

		pathBytes := []byte(path)
		pathLen := make([]byte, 2)
		binary.LittleEndian.PutUint16(pathLen, uint16(len(pathBytes)))

		var msgpData []byte
		msgpData = msgp.AppendArrayHeader(msgpData, uint32(len(entries)))
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			item := biz.FileInfo{
				Path:  filepath.Join(path, entry.Name()),
				Size:  info.Size(),
				Mode:  uint32(info.Mode()),
				Mtime: info.ModTime().Unix(),
			}
			msgpData, _ = item.MarshalMsg(msgpData)
		}
		s.Write(utils.JoinBytes2(0x13, pathLen, pathBytes, msgpData))
	}

	// delete file or directory
	s.Handlers[0x14] = func(recv []byte) {
		path := string(recv[1:])
		if err := os.RemoveAll(path); err != nil {
			s.WriteDebugMessage(err.Error())
			return
		}
		s.Write(utils.PrependBytes([]byte{0x14}, []byte(path)))
	}

	// create directory
	s.Handlers[0x15] = func(recv []byte) {
		path := string(recv[1:])
		if err := os.MkdirAll(path, 0755); err != nil {
			s.WriteDebugMessage(err.Error())
			return
		}
		s.Write(utils.PrependBytes([]byte{0x15}, []byte(path)))
	}

	// read file chunk
	s.Handlers[0x12] = func(recv []byte) {
		offset := int64(binary.LittleEndian.Uint64(recv[1:]))
		length := int64(binary.LittleEndian.Uint64(recv[9:]))
		file_path := string(recv[17:])

		data, err := read_file_chunk(file_path, offset, length)
		if data == nil {
			s.WriteDebugMessage(err.Error())
			return
		}

		actual_length := int64(len(data))
		s.Write(utils.JoinBytes2(
			0x12,
			binary.LittleEndian.AppendUint64(nil, uint64(offset)),
			binary.LittleEndian.AppendUint64(nil, uint64(actual_length)),
			[]byte(file_path),
			data,
		))
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
