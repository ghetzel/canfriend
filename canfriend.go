package main

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/brutella/can"
	"github.com/ghetzel/sortedset"
)

var DefaultFrameEmitTimeout = 100 * time.Millisecond
var DefaultFrameSummaryLimit = 20

type SortKey string

const (
	SortByID       SortKey = `id`
	SortByCount            = `count`
	SortByLength           = `length`
	SortByLastSeen         = `lastseen`
)

type Analyzer struct {
	FrameSummaryLimit        int
	device                   string
	canbus                   *can.Bus
	frames                   chan *can.Frame
	eventSummariesById       *sortedset.SortedSet
	eventSummariesByCount    *sortedset.SortedSet
	eventSummariesByLength   *sortedset.SortedSet
	eventSummariesByLastSeen *sortedset.SortedSet
	summaryLock              sync.Mutex
}

type FrameSummary struct {
	Key           string
	Count         int
	LatestFrame   *can.Frame
	PreviousFrame *can.Frame
	LastSeen      time.Time
}

func NewAnalyzer(device string) *Analyzer {
	return &Analyzer{
		FrameSummaryLimit:        DefaultFrameSummaryLimit,
		device:                   device,
		eventSummariesById:       sortedset.New(),
		eventSummariesByLength:   sortedset.New(),
		eventSummariesByCount:    sortedset.New(),
		eventSummariesByLastSeen: sortedset.New(),
	}
}

func (self *Analyzer) Run() error {
	self.frames = make(chan *can.Frame)

	if iface, err := net.InterfaceByName(self.device); err == nil {
		if rwc, err := can.NewReadWriteCloserForInterface(iface); err == nil {
			self.canbus = can.NewBus(rwc)
		} else {
			return err
		}
	} else {
		return err
	}

	self.canbus.SubscribeFunc(self.handleFrame)

	if err := self.canbus.ConnectAndPublish(); err != nil {
		if !strings.HasSuffix(err.Error(), `bad file descriptor`) {
			return err
		}
	}

	return nil
}

func (self *Analyzer) Stop() error {
	log.Debugf("Stopping Analyzer")
	return self.canbus.Disconnect()
}

func (self *Analyzer) Frames() <-chan *can.Frame {
	return self.frames
}

func (self *Analyzer) GetFrameSummary(orderBy SortKey, reverse bool) []*FrameSummary {
	summary := make([]*FrameSummary, 0)
	var set *sortedset.SortedSet

	switch orderBy {
	case SortByID:
		set = self.eventSummariesById
	case SortByCount:
		set = self.eventSummariesByCount
	case SortByLength:
		set = self.eventSummariesByLength
	case SortByLastSeen:
		set = self.eventSummariesByLastSeen
	}

	nodes := set.GetByRankRange(1, -1, false)

	if reverse {
		for i := len(nodes); i > 0; i-- {
			node := nodes[i-1]

			if rollup, ok := node.Value.(*FrameSummary); ok {
				summary = append(summary, rollup)
			}
		}
	} else {
		for i := 0; i < len(nodes); i++ {
			node := nodes[i]

			if rollup, ok := node.Value.(*FrameSummary); ok {
				summary = append(summary, rollup)
			}
		}
	}

	return summary
}

func (self *Analyzer) ClearSummary() {
	self.summaryLock.Lock()
	defer self.summaryLock.Unlock()

	self.eventSummariesById.GetByRankRange(1, -1, true)
	self.eventSummariesByCount.GetByRankRange(1, -1, true)
	self.eventSummariesByLength.GetByRankRange(1, -1, true)
	self.eventSummariesByLastSeen.GetByRankRange(1, -1, true)
}

func (self *Analyzer) trimSummary(maxSize int) {
	if self.eventSummariesByLastSeen.GetCount() >= maxSize {
		removals := self.eventSummariesByLastSeen.GetByRankRange(maxSize, -1, true)

		for _, node := range removals {
			key := node.Key()

			self.eventSummariesById.Remove(key)
			self.eventSummariesByCount.Remove(key)
			self.eventSummariesByLength.Remove(key)
		}
	}
}

func (self *Analyzer) handleFrame(frame can.Frame) {
	self.storeFrameSummary(&frame, frame.ID)

	select {
	case self.frames <- &frame:
	default:
	}
}

func (self *Analyzer) getScoreFor(orderBy SortKey, summary *FrameSummary) int {
	switch orderBy {
	case SortByID:
		return int(summary.LatestFrame.ID)
	case SortByCount:
		return int(summary.Count)
	case SortByLength:
		return int(summary.LatestFrame.Length)
	case SortByLastSeen:
		return int(summary.LastSeen.UnixNano()) / int(time.Millisecond)
	}

	return -1
}

func (self *Analyzer) storeFrameSummary(frame *can.Frame, keyField interface{}) {
	self.summaryLock.Lock()
	defer self.summaryLock.Unlock()

	var previousFrame *can.Frame

	frameKey := fmt.Sprintf("%x", keyField)
	count := 0

	// get an existing FrameSummary for this key (all of the sets should hold
	// the same pointer)
	if node := self.eventSummariesByLastSeen.GetByKey(frameKey); node != nil {
		if fs, ok := node.Value.(*FrameSummary); ok {
			count = fs.Count
			previousFrame = fs.LatestFrame
		}
	}

	count += 1

	summary := &FrameSummary{
		Key:           frameKey,
		Count:         count,
		LatestFrame:   frame,
		PreviousFrame: previousFrame,
		LastSeen:      time.Now(),
	}

	self.trimSummary(self.FrameSummaryLimit)

	for score, set := range map[int]*sortedset.SortedSet{
		self.getScoreFor(SortByID, summary): self.eventSummariesById,
		count: self.eventSummariesByCount,
		self.getScoreFor(SortByLength, summary):   self.eventSummariesByLength,
		self.getScoreFor(SortByLastSeen, summary): self.eventSummariesByLastSeen,
	} {
		set.AddOrUpdate(frameKey, sortedset.SCORE(score), summary)
	}
}
