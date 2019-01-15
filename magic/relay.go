package magic

import (
	"encoding/binary"
	"io"
	"net"
	"time"
)

// Relay data from local to remote
// On error stop both connection
func localToRemote(localConn, remoteConn net.Conn) chan bool {
	exit := make(chan bool, 2)
	go func(quit chan bool) {
		io.Copy(remoteConn, localConn)
		remoteConn.SetDeadline(time.Now())
		localConn.SetDeadline(time.Now())
		quit <- true
	}(exit)
	return exit
}

// Receive data from the connection
func bufferFromRemote(conn net.Conn, dataBlocks chan dataBlock) (chan bool, chan bool) {
	exitSignal := make(chan bool, 2)
	taskExit := make(chan bool)
	go func() {
		for {
			var metaData [8]byte
			n, err := io.ReadFull(conn, metaData[:])
			if err != nil || n != 8 {
				break
			}
			blockID := binary.LittleEndian.Uint32(metaData[:])
			blockSize := binary.LittleEndian.Uint32(metaData[4:])

			blockData := make([]byte, blockSize)
			if blockSize != 0 {
				n, err = io.ReadFull(conn, blockData)
				if err != nil {
					break
				}
			}
			select {
			case dataBlocks <- dataBlock{
				blockData,
				blockSize,
				blockID,
			}:
				continue
			case <-exitSignal:
				taskExit <- true
				return
			}
		}
		taskExit <- true
	}()
	return taskExit, exitSignal
}
