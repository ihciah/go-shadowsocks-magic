package magic

import (
	"log"
	"net"
	"time"
)

// Fetch remote resource into GBT and treat the main connection as a normal child connection
func RelayRemoteMain(localConn, remoteConn net.Conn, GBT *GlobalBufferTable) {
	k := GBT.New()
	defer GBT.Free(k)
	_, err := localConn.Write(k[:])
	if err != nil {
		return
	}

	defer func() {
		// Broadcast exit(peaceful) signals to all receivers
		for _, receiver := range (*GBT)[k].ExitSignals {
			receiver <- false
		}
		// Wait for receivers exit
		(*GBT)[k].WG.Wait()
		remoteConn.SetDeadline(time.Now())
		localConn.SetDeadline(time.Now())
	}()

	lrExit := localToRemote(localConn, remoteConn)
	go bufferToLocal(localConn, (*GBT)[k])

	// Receive data from remote and push it to channel
	var bID uint32
	for bID = 0; ; bID++ {
		// Prevent id overflow
		bID = bID % TableSize
		dataBlock := DataBlock{
			make([]byte, bufSize),
			0,
			bID,
		}
		n, err := remoteConn.Read(dataBlock.Data)
		if err != nil {
			log.Printf("Error when read remote: %s\n", err)
			break
		}
		dataBlock.Size = uint32(n)
		select {
		case (*GBT)[k].Chan <- dataBlock:
			continue
		case <-lrExit:
			return
		}
	}
	return
}

// Send block data
func RelayRemoteChild(localConnChild net.Conn, dataKey [16]byte, GBT *GlobalBufferTable) (int64, error) {
	log.Printf("Child thread start.\n")
	bufferNode, ok := (*GBT)[dataKey]
	if !ok {
		log.Printf("DataKey Invalid.\n")
		return 0, INVALID_KEY
	}
	log.Printf("DataKey Verified.\n")
	bufferToLocal(localConnChild, bufferNode)
	return 0, nil
}

// Relay data from GBT to local
// On error or exit signal return
func bufferToLocal(conn net.Conn, bufferNode *BufferNode) {
	exitSignal := make(chan bool, 2)
	bufferNode.Lock.Lock()
	bufferNode.ExitSignals = append(bufferNode.ExitSignals, exitSignal)
	bufferNode.Lock.Unlock()
	bufferNode.WG.Add(1)

	defer bufferNode.WG.Done()
	for {
		select {
		case dataBlock := <-bufferNode.Chan:
			bytes := dataBlock.Pack()
			_, err := conn.Write(bytes)
			if err != nil {
				log.Printf("Error when sending packet: %s\n", err)
				bufferNode.Chan <- dataBlock
				return
			}
		case s := <-exitSignal:
			if s == false {
				// finish all tasks first before leave
				for {
					select {
					case dataBlock := <-bufferNode.Chan:
						bytes := dataBlock.Pack()
						_, err := conn.Write(bytes)
						if err != nil {
							log.Printf("Error when sending packet: %s\n", err)
							bufferNode.Chan <- dataBlock
							return
						}
					default:
						return
					}
				}
			} else {
				// exit directly
				return
			}
		}
	}
}
