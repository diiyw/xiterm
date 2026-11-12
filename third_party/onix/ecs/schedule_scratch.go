package ecs

import "sync"

type scheduleNode struct {
	name     string
	inDegree int
	edge     int // head of the outgoing edge list; -1 means no edges
}

type scheduleEdge struct {
	target int
	next   int
}

type scheduleScratch struct {
	indices map[string]int
	nodes   []scheduleNode
	edges   []scheduleEdge
	ready   minHeap[string]
}

var scheduleScratchPool sync.Pool

func borrowScheduleScratch() *scheduleScratch {
	scratch, _ := scheduleScratchPool.Get().(*scheduleScratch)
	if scratch == nil {
		scratch = &scheduleScratch{}
	}
	if scratch.indices == nil {
		scratch.indices = make(map[string]int)
	}
	return scratch
}

func (s *scheduleScratch) release() {
	const maxSystems = 4096
	if len(s.indices) > maxSystems {
		s.indices = nil
	} else {
		clear(s.indices)
	}
	if cap(s.nodes) > maxSystems {
		s.nodes = nil
	} else {
		clear(s.nodes)
		s.nodes = s.nodes[:0]
	}
	if cap(s.edges) > maxSystems*4 {
		s.edges = nil
	} else {
		s.edges = s.edges[:0]
	}
	if cap(s.ready) > maxSystems {
		s.ready = nil
	} else {
		clear(s.ready)
		s.ready = s.ready[:0]
	}
	scheduleScratchPool.Put(s)
}

func (s *scheduleScratch) addEdge(source, target int) {
	s.edges = append(s.edges, scheduleEdge{target: target, next: s.nodes[source].edge})
	s.nodes[source].edge = len(s.edges) - 1
	s.nodes[target].inDegree++
}
