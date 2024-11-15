// Copyright (c) 2018 The MATRIX Authors
// Distributed under the MIT software license, see the accompanying
// file COPYING or http://www.opensource.org/licenses/mit-license.php

package miner

import (
	"sync"
	"sync/atomic"

	"github.com/MatrixAINetwork/go-matrix/consensus"
	"github.com/MatrixAINetwork/go-matrix/log"
	"github.com/MatrixAINetwork/go-matrix/params/manversion"
)

type CpuAgent struct {
	mu sync.Mutex

	workCh chan *Work
	stop   chan struct{}

	quitCurrentOp chan struct{}
	returnCh      chan<- *consensus.SealResult

	chain ChainReader

	isMining int32 // isMining indicates whether the agent is currently mining
}

func NewCpuAgent(chain ChainReader) *CpuAgent {
	miner := &CpuAgent{
		chain:  chain,
		stop:   make(chan struct{}, 1),
		workCh: make(chan *Work, 1),
	}
	return miner
}

func (self *CpuAgent) Work() chan<- *Work                          { return self.workCh }
func (self *CpuAgent) SetReturnCh(ch chan<- *consensus.SealResult) { self.returnCh = ch }

func (self *CpuAgent) Stop() {
	if !atomic.CompareAndSwapInt32(&self.isMining, 1, 0) {
		return // agent already stopped
	}
	self.stop <- struct{}{}
done:
	// Empty work channel
	for {
		select {
		case <-self.workCh:
		default:
			break done
		}
	}
}

func (self *CpuAgent) Start() {
	if !atomic.CompareAndSwapInt32(&self.isMining, 0, 1) {
		return // agent already started
	}

	go self.update()
}

func (self *CpuAgent) update() {
out:
	for {
		select {
		case work := <-self.workCh:
			self.mu.Lock()
			if self.quitCurrentOp != nil {
				close(self.quitCurrentOp)
			}
			self.quitCurrentOp = make(chan struct{})
			go self.mine(work, self.quitCurrentOp)
			self.mu.Unlock()
		case <-self.stop:
			self.mu.Lock()
			if self.quitCurrentOp != nil {
				close(self.quitCurrentOp)
				self.quitCurrentOp = nil
			}
			self.mu.Unlock()
			log.Info("miner", "CpuAgent Stop Minning", "")
			break out
		}
	}
}

func (self *CpuAgent) mine(work *Work, stop <-chan struct{}) {
	switch work.mineType {
	case mineTaskTypePow:
		if manversion.VersionCmp(string(work.header.Version), manversion.VersionAIMine) >= 0 {
			self.chain.Engine(work.header.Version).SealPow(self.chain, work.header, stop, self.returnCh, work.isBroadcastNode)
		} else {
			if result, err := self.chain.Engine(work.header.Version).SealPow(self.chain, work.header, stop, self.returnCh, work.isBroadcastNode); result != nil {
				log.Info("Successfully sealed new block", "number", result.Header.Number, "hash", result.Header.Hash())
				self.returnCh <- result
			} else {
				if err != nil {
					log.Warn("Block sealing failed", "err", err)
				}
				self.returnCh <- nil
			}
		}
	case mineTaskTypeAI:
		if result, err := self.chain.Engine(work.header.Version).SealAI(self.chain, work.header, stop); result != nil {
			log.Info("cup agent", "Successfully sealed new AI result", result.Header.AIHash.TerminalString(), "number", result.Header.Number)
			self.returnCh <- result
		} else {
			if err != nil {
				log.Warn("cup agent", "ai sealing failed", err)
			}
			self.returnCh <- nil
		}
	default:
		log.Warn("cpu agent", "unknown mine type", work.mineType)
	}
}

func (self *CpuAgent) GetHashRate() int64 {
	if pow, ok := self.chain.Engine([]byte(manversion.VersionAlpha)).(consensus.PoW); ok {
		return int64(pow.Hashrate())
	}
	return 0
}
