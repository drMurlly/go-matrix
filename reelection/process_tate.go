// Copyright (c) 2018 The MATRIX Authors
// Distributed under the MIT software license, see the accompanying
// file COPYING or http://www.opensource.org/licenses/mit-license.php
package reelection

import (
	"errors"

	"github.com/MatrixAINetwork/go-matrix/common"
	"github.com/MatrixAINetwork/go-matrix/core"
	"github.com/MatrixAINetwork/go-matrix/core/state"
	"github.com/MatrixAINetwork/go-matrix/core/types"
	"github.com/MatrixAINetwork/go-matrix/log"
	"github.com/MatrixAINetwork/go-matrix/mc"
)

func (self *ReElection) ProduceElectGraphData(block *types.Block, stateDb *state.StateDBManage, readFn core.PreStateReadFn) (interface{}, error) {
	log.Trace(Module, "ProduceElectGraphData", "start", "height", block.Header().Number.Uint64())
	defer log.Trace(Module, "ProduceElectGraphData", "end", "height", block.Header().Number.Uint64())
	if err := CheckBlock(block); err != nil {
		log.Error(Module, "ProduceElectGraphData CheckBlock err ", err)
		return nil, err
	}
	electStates, bcInterval, genData, err := self.getElectStates(readFn, block)
	if nil != err {
		return nil, err
	}

	height := block.Header().Number.Uint64()
	electStates.Number = height

	if self.IsMinerTopGenTiming(bcInterval, height, genData) {
		currentHash := block.ParentHash()
		log.Info(Module, "计算矿工拓扑计算 ", currentHash.String())
		electStates.NextMinerElect = []mc.ElectNodeInfo{}

		MastM, BackM, CandM, err := self.ToGenMinerTop(currentHash, stateDb)
		if err != nil {
			log.Error(Module, "矿工拓扑生成错误 err", err)
			return nil, err
		} else {
			electStates.NextMinerElect = append(electStates.NextMinerElect, MastM...)
			electStates.NextMinerElect = append(electStates.NextMinerElect, BackM...)
			electStates.NextMinerElect = append(electStates.NextMinerElect, CandM...)
		}

	}
	if self.IsValidatorTopGenTiming(bcInterval, height, genData) {
		electStates.NextValidatorElect = []mc.ElectNodeInfo{}
		currentHash := block.ParentHash()
		log.Info(Module, "计算验证者拓扑计算 ", currentHash)
		MastV, BackV, CandV, err := self.ToGenValidatorTop(currentHash, stateDb)
		if err != nil {
			log.Error(Module, "验证者拓扑生成错误 err", err)
			return nil, err
		} else {
			electStates.NextValidatorElect = append(electStates.NextValidatorElect, MastV...)
			electStates.NextValidatorElect = append(electStates.NextValidatorElect, BackV...)
			electStates.NextValidatorElect = append(electStates.NextValidatorElect, CandV...)
		}

	}

	if bcInterval.IsReElectionNumber(block.NumberU64() + 1) {
		nextElect := electStates.NextMinerElect
		nextElect = append(nextElect, electStates.NextValidatorElect...)
		electList := []mc.ElectNodeInfo{}
		for _, v := range nextElect {
			switch v.Type {
			case common.RoleBackupValidator:
				electList = append(electList, v)
			case common.RoleValidator:
				electList = append(electList, v)
			case common.RoleMiner:
				electList = append(electList, v)
			case common.RoleCandidateValidator:
				electList = append(electList, v)
			}
		}
		electStates.ElectList = append([]mc.ElectNodeInfo{}, electList...)
		electStates.NextMinerElect = []mc.ElectNodeInfo{}
		electStates.NextValidatorElect = []mc.ElectNodeInfo{}
	}
	//log.DEBUG(Module, "高度", block.Number().Uint64(), "ProduceElectGraphData data", electStates)
	return electStates, nil
}

func (self *ReElection) IsValidatorTopGenTiming(bcInterval *mc.BCIntervalInfo, height uint64, genData *mc.ElectGenTimeStruct) bool {
	return bcInterval.IsReElectionNumber(height + uint64(genData.ValidatorNetChange))
}

func (self *ReElection) IsMinerTopGenTiming(bcInterval *mc.BCIntervalInfo, height uint64, genData *mc.ElectGenTimeStruct) bool {
	return bcInterval.IsReElectionNumber(height + uint64(genData.MinerNetChange))
}

func (self *ReElection) getElectStates(readFn core.PreStateReadFn, block *types.Block) (*mc.ElectGraph, *mc.BCIntervalInfo, *mc.ElectGenTimeStruct, error) {
	data, err := readFn(mc.MSKeyElectGraph)
	if err != nil {
		log.Error(Module, "readFn 失败 key", mc.MSKeyElectGraph, "err", err)
		return nil, nil, nil, err
	}
	electStates, OK := data.(*mc.ElectGraph)
	if OK == false || electStates == nil {
		log.Error(Module, "ElectStates 非法", "反射失败")
		return nil, nil, nil, errors.New("ElectStates 反射失败")
	}

	bciData, err := readFn(mc.MSKeyBroadcastInterval)
	if err != nil {
		log.Error(Module, "ProducePreAllTopData read broadcast interval err", err)
		return nil, nil, nil, err
	}
	bcInterval, OK := bciData.(*mc.BCIntervalInfo)
	if !OK || bcInterval == nil {
		log.Error(Module, "ProducePreAllTopData broadcast interval reflect err", err)
		return nil, nil, nil, errors.New("ProducePreAllTopData broadcast interval reflect err")
	}
	genOldData, err := readFn(mc.MSKeyElectGenTime)
	if err != nil {
		log.Error(Module, "ProducePreAllTopData read broadcast interval err", err)
		return nil, nil, nil, err
	}
	genData, OK := genOldData.(*mc.ElectGenTimeStruct)
	if !OK || genData == nil {
		log.Error(Module, "ProducePreAllTopData broadcast interval reflect err", err)
		return nil, nil, nil, errors.New("ProducePreAllTopData broadcast interval reflect err")
	}
	return electStates, bcInterval, genData, nil
}

func (self *ReElection) ProduceElectOnlineStateData(block *types.Block, stateDb *state.StateDBManage, readFn core.PreStateReadFn) (interface{}, error) {
	log.Trace(Module, "ProduceElectOnlineStateData", "start", "height", block.Header().Number.Uint64())
	defer log.Trace(Module, "ProduceElectOnlineStateData", "end", "height", block.Header().Number.Uint64())
	if err := CheckBlock(block); err != nil {
		log.Error(Module, "ProduceElectGraphData CheckBlock err ", err)
		return nil, err
	}

	height := block.Header().Number.Uint64()

	bciData, err := readFn(mc.MSKeyBroadcastInterval)
	if err != nil {
		log.Error(Module, "ProduceElectOnlineStateData read broadcast interval err", err)
		return nil, err
	}
	bcInterval, OK := bciData.(*mc.BCIntervalInfo)
	if OK == false {
		log.Error(Module, "ProduceElectOnlineStateData broadcast interval reflect err", err)
		return nil, errors.New("broadcast interval reflect failed")
	}

	if bcInterval.IsReElectionNumber(height + 1) {
		electOnline := &mc.ElectOnlineStatus{
			Number: height,
		}

		electData, err := readFn(mc.MSKeyElectGraph)
		if err != nil {
			log.Error(Module, "ProduceElectOnlineStateData read preElectGraph err", err)
			return nil, err
		}
		electGraph, OK := electData.(*mc.ElectGraph)
		if OK == false {
			log.Error(Module, "ProduceElectOnlineStateData preElectGraph reflect failed", err)
			return nil, errors.New("preElectGraph reflect failed")
		}
		masterV, backupV, CandV, err := self.GetNextElectNodeInfo(electGraph, common.RoleValidator)
		if err != nil {
			log.Error(Module, "获取验证者全拓扑图失败 err", err)
			return nil, err
		}
		for _, v := range masterV {
			tt := v
			tt.Position = common.PosOnline
			electOnline.ElectOnline = append(electOnline.ElectOnline, tt)
		}
		for _, v := range backupV {
			tt := v
			tt.Position = common.PosOnline
			electOnline.ElectOnline = append(electOnline.ElectOnline, tt)
		}
		for _, v := range CandV {
			tt := v
			tt.Position = common.PosOnline
			electOnline.ElectOnline = append(electOnline.ElectOnline, tt)
		}
		log.Debug(Module, "高度", block.Number().Uint64(), "ProduceElectOnlineStateData data", electOnline)
		return electOnline, nil
	}

	header := block.Header()
	data, err := readFn(mc.MSKeyElectOnlineState)
	//log.INFO(Module, "data", data, "err", err)
	if err != nil {
		log.Error(Module, "readFn 失败 key", mc.MSKeyElectOnlineState, "err", err)
		return nil, err
	}
	electStates, OK := data.(*mc.ElectOnlineStatus)
	if OK == false || electStates == nil {
		log.Error(Module, "ElectStates 非法", "反射失败")
		return nil, err
	}
	mappStatus := make(map[common.Address]uint16)
	for _, v := range header.NetTopology.NetTopologyData {
		switch v.Position {
		case common.PosOnline:
			mappStatus[v.Account] = common.PosOnline
		case common.PosOffline:
			mappStatus[v.Account] = common.PosOffline
		}
	}
	for k, v := range electStates.ElectOnline {
		if _, ok := mappStatus[v.Account]; ok == false {
			continue
		}
		electStates.ElectOnline[k].Position = mappStatus[v.Account]
	}

	log.Debug(Module, "高度", block.Number().Uint64(), "ProduceElectOnlineStateData data", electStates)
	return electStates, nil
}

func (self *ReElection) ProducePreBroadcastStateData(block *types.Block, stateDb *state.StateDBManage, readFn core.PreStateReadFn) (interface{}, error) {
	if err := CheckBlock(block); err != nil {
		log.Error(Module, "ProducePreBroadcastStateData CheckBlock err ", err)
		return []byte{}, err
	}
	bciData, err := readFn(mc.MSKeyBroadcastInterval)
	if err != nil {
		log.Error(Module, "ProducePreBroadcastStateData read broadcast interval err", err)
		return nil, err
	}
	bcInterval, OK := bciData.(*mc.BCIntervalInfo)
	if err != nil {
		log.Error(Module, "ProducePreBroadcastStateData broadcast interval reflect err", err)
	}
	height := block.Header().Number.Uint64()
	if height == 1 {
		firstData := &mc.PreBroadStateRoot{
			LastStateRoot:       make([]common.CoinRoot, 0),
			BeforeLastStateRoot: make([]common.CoinRoot, 0),
		}
		return firstData, nil
	}

	if bcInterval.IsBroadcastNumber(height-1) == false {
		return nil, nil
	}
	data, err := readFn(mc.MSKeyPreBroadcastRoot)
	if err != nil {
		log.Error(Module, "readFn 失败 key", mc.MSKeyPreBroadcastRoot, "err", err)
		return nil, err
	}
	preBroadcast, OK := data.(*mc.PreBroadStateRoot)
	if OK == false || preBroadcast == nil {
		log.Error(Module, "PreBroadStateRoot 非法", "反射失败")
		return nil, err
	}
	header := self.bc.GetHeaderByHash(block.ParentHash())
	if header == nil {
		log.Error(Module, "根据hash算区块头失败 高度", block.Number().Uint64())
		return nil, errors.New("header is nil")
	}
	preBroadcast.BeforeLastStateRoot = make([]common.CoinRoot, len(preBroadcast.LastStateRoot))
	copy(preBroadcast.BeforeLastStateRoot, preBroadcast.LastStateRoot)
	preBroadcast.LastStateRoot = make([]common.CoinRoot, len(header.Roots))
	copy(preBroadcast.LastStateRoot, header.Roots)
	//log.INFO(Module, "高度", block.Number().Uint64(), "ProducePreBroadcastStateData beforelast", preBroadcast.BeforeLastStateRoot, "last", preBroadcast.LastStateRoot)

	return preBroadcast, nil

}
func (self *ReElection) ProduceMinHashData(block *types.Block, stateDb *state.StateDBManage, readFn core.PreStateReadFn) (interface{}, error) {
	if err := CheckBlock(block); err != nil {
		log.Error(Module, "ProduceMinHashData CheckBlock err ", err)
		return []byte{}, err
	}
	bciData, err := readFn(mc.MSKeyBroadcastInterval)
	if err != nil {
		log.Error(Module, "ProduceMinHashData read broadcast interval err", err)
		return nil, err
	}
	bcInterval, OK := bciData.(*mc.BCIntervalInfo)
	if err != nil {
		log.Error(Module, "ProduceMinHashData broadcast interval reflect err", err)
	}
	height := block.Number().Uint64()
	preHeader := self.bc.GetHeaderByHash(block.ParentHash())
	if preHeader == nil {
		log.Error(Module, "根据hash算区块头失败 高度", block.Number().Uint64())
		return nil, errors.New("header is nil")
	}
	if bcInterval.IsBroadcastNumber(height - 1) {
		log.Info(Module, "ProduceMinHashData", "是广播区块后一块", "高度", height)
		return &mc.RandomInfoStruct{MinHash: block.ParentHash(), MaxNonce: preHeader.Nonce.Uint64()}, nil
	}
	data, err := readFn(mc.MSKeyMinHash)
	if err != nil {
		log.Error(Module, "readFn 失败 key", mc.MSKeyMinHash, "err", err)
		return nil, err
	}
	randomInfo, OK := data.(*mc.RandomInfoStruct)
	if OK == false || randomInfo == nil {
		log.Error(Module, "PreBroadStateRoot 非法", "反射失败")
		return nil, err
	}

	nowHash := preHeader.Hash().Big()
	if nowHash.Cmp(randomInfo.MinHash.Big()) < 0 {
		randomInfo.MinHash = preHeader.Hash()
	}
	if preHeader.Nonce.Uint64() > randomInfo.MaxNonce {
		randomInfo.MaxNonce = preHeader.Nonce.Uint64()
	}
	//log.INFO(Module, "高度", block.Number().Uint64(), "ProduceMinHashData", randomInfo.MinHash.String())
	return randomInfo, nil
}

/*func (self *ReElection) ProducePreAllTopData(block *types.Block, readFn matrixstate.PreStateReadFn) (interface{}, error) {

	if err := CheckBlock(block); err != nil {
		log.ERROR(Module, "ProducePreAllTopData CheckBlock err ", err)
		return []byte{}, err
	}
	log.INFO(Module, "ProducePreAllTopData ", "开始", "高度", block.Header().Number.Uint64())
	defer log.INFO(Module, "ProducePreAllTopData ", "结束", "高度", block.Header().Number.Uint64())
	bciData, err := readFn(mc.MSKeyBroadcastInterval)
	if err != nil {
		log.Error(Module, "ProducePreAllTopData read broadcast interval err", err)
		return nil, err
	}
	bcInterval, err := manparams.NewBCIntervalWithInterval(bciData)
	if err != nil {
		log.Error(Module, "ProducePreAllTopData create broadcast interval err", err)
	}
	height := block.Header().Number.Uint64()
	if bcInterval.IsReElectionNumber(height) == false {
		return nil, nil
	}

	header := self.bc.GetHeaderByHash(block.ParentHash())
	if header == nil {
		log.ERROR(Module, "根据hash算区块头失败 高度", block.Number().Uint64())
		return nil, errors.New("header is nil")
	}
	preAllTop := &mc.PreAllTopStruct{}
	preAllTop.PreAllTopRoot = header.Roots
	log.INFO("高度", block.Number().Uint64(), "ProducePreAllTopData", "preAllTop.PreAllTopRoot", preAllTop.PreAllTopRoot)
	return preAllTop, nil
}
*/
