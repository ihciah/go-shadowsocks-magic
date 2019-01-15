package magic

import (
	"net"
	"time"
)

// Start connections to download data in parallel and join them together
func RelayLocal(localConn, remoteConn net.Conn, createConn func([16]byte) net.Conn) {
	lrExit := localToRemote(localConn, remoteConn)
	var dataKey [16]byte
	n, err := remoteConn.Read(dataKey[:])
	if err != nil || n != 16 {
		return
	}

	dataBlocks, continuousData, exitJoinBlock, joinBlockfinish := blockJoiner()
	defer func() { exitJoinBlock <- true }()

	exitThreadMan := threadManager(createConn, dataBlocks, dataKey)
	recvExit, exitRecv := bufferFromRemote(remoteConn, dataBlocks)
	sendExit, exitSend := dataBlockToConn(localConn, continuousData)

	leave := func() {
		remoteConn.SetDeadline(time.Now())
		localConn.SetDeadline(time.Now())
		exitRecv <- true
		exitSend <- true
		exitThreadMan <- true
	}

	select {
	case <-lrExit:
		leave()
		return
	case <-sendExit:
		leave()
		return
	case <-recvExit:
		// Wait for data process finished or leave
		exitThreadMan <- true
		exitJoinBlock <- false
		exitSend <- false
		<-joinBlockfinish
		<-sendExit
	}
	return
}

func relayLocalChild(createConn func([16]byte) net.Conn, dataBlocks chan dataBlock, dataKey [16]byte, exit chan bool) {
	conn := createConn(dataKey)
	if conn == nil {
		return
	}
	defer conn.Close()
	taskExit, exitSignal := bufferFromRemote(conn, dataBlocks)
	for {
		select {
		case <-taskExit:
			return
		case e := <-exit:
			exitSignal <- e
			return
		}
	}
}

// Connection Manager: create 4 connections every 0.5 seconds until Max.
func threadManager(createConn func([16]byte) net.Conn, dataBlocks chan dataBlock, dataKey [16]byte) chan bool {
	exitSignal := make(chan bool, 2)
	go func() {
		exitSignals := make([]chan bool, 0)
		for {
			select {
			case <-time.After(time.Duration(time.Millisecond) * 500):
				currentConn := len(exitSignals)
				for i := currentConn; i < maxConnection && i < currentConn+2; i++ {
					exitS := make(chan bool, 2)
					exitSignals = append(exitSignals, exitS)
					go relayLocalChild(createConn, dataBlocks, dataKey, exitS)
				}
			case eSignal := <-exitSignal:
				for _, s := range exitSignals {
					s <- eSignal
				}
				return
			}
		}
	}()
	return exitSignal
}

// Write dataBlock to connection
func dataBlockToConn(conn net.Conn, db chan dataBlock) (chan bool, chan bool) {
	taskExit := make(chan bool, 2)
	exitSignal := make(chan bool, 2)
	go func() {
		for {
			select {
			case data := <-db:
				_, err := conn.Write(data.Data)
				if err != nil {
					taskExit <- true
					return
				}
			case s := <-exitSignal:
				if s {
					return
				}
				for {
					select {
					case data := <-db:
						_, err := conn.Write(data.Data)
						if err != nil {
							taskExit <- true
							return
						}
					default:
						return
					}
				}
			}
		}
	}()
	return taskExit, exitSignal
}
