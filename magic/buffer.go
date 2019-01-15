package magic

import (
	"crypto/rand"
	"encoding/binary"
	"io"
	"sync"
)

const maxConnection = 16
const bufSize = 64*1024 - 8
const tableSize = 65536

type GlobalBufferTable map[[16]byte](*bufferNode)

type bufferNode struct {
	Chan        chan dataBlock
	WG          sync.WaitGroup
	ExitSignals []chan bool
	Lock        sync.Mutex
}

func makeBufferNode() bufferNode {
	var wg sync.WaitGroup
	var lock sync.Mutex
	return bufferNode{
		make(chan dataBlock, maxConnection*2),
		wg,
		make([]chan bool, 0),
		lock,
	}
}

type dataBlock struct {
	Data    []byte
	Size    uint32
	BlockId uint32
}

func (dataBlock dataBlock) Pack() []byte {
	packedData := make([]byte, 8+dataBlock.Size)
	binary.LittleEndian.PutUint32(packedData[:], dataBlock.BlockId)
	binary.LittleEndian.PutUint32(packedData[4:], dataBlock.Size)
	copy(packedData[8:], dataBlock.Data)
	return packedData
}

// Create a new key-value and return the key
func (gbt *GlobalBufferTable) New() [16]byte {
	var key [16]byte
	for {
		io.ReadFull(rand.Reader, key[:])
		if _, exist := (*gbt)[key]; !exist {
			bufferNode := makeBufferNode()
			(*gbt)[key] = &bufferNode
			return key
		}
	}
}

// Delete a key-value
func (gbt *GlobalBufferTable) Free(key [16]byte) {
	if _, ok := (*gbt)[key]; !ok {
		return
	}
	delete(*gbt, key)
}

func joinBlocks(inData, outData chan dataBlock, exitSignal, taskFinish chan bool) {
	table := make(map[uint32]dataBlock)
	var pointer uint32 = 0
	for {
		select {
		case db := <-inData:
			table[db.BlockId%tableSize] = db
			if pointer != db.BlockId%tableSize {
				continue
			}
			for {
				if d, exist := table[pointer]; exist {
					outData <- d
					delete(table, pointer)
					pointer = (pointer + 1) % tableSize
					continue
				}
				break
			}
		case s := <-exitSignal:
			if s {
				return
			}
			for {
				select {
				case db := <-inData:
					table[db.BlockId%tableSize] = db
					if pointer != db.BlockId%tableSize {
						continue
					}
					for {
						if d, exist := table[pointer]; exist {
							outData <- d
							delete(table, pointer)
							pointer = (pointer + 1) % tableSize
							continue
						}
						break
					}
				default:
					taskFinish <- true
					return
				}
			}
		}
	}
}

func blockJoiner() (chan dataBlock, chan dataBlock, chan bool, chan bool) {
	dataBlocks := make(chan dataBlock, maxConnection*2)
	continuousData := make(chan dataBlock, maxConnection*2)
	exitJoinBlock := make(chan bool, 2)
	finishSignal := make(chan bool, 2)
	go joinBlocks(dataBlocks, continuousData, exitJoinBlock, finishSignal)
	return dataBlocks, continuousData, exitJoinBlock, finishSignal
}
