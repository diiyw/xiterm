package ecs

import (
	"fmt"
	"reflect"
)

type System func(*SystemContext) error

type Condition func(*SystemContext) bool

type SystemContext struct {
	World    *World
	Commands *Commands
	locals   map[reflect.Type]any
	lastRun  Tick
	thisRun  Tick
}

// ChangesSince snapshots a read window ending at the current change tick.
// It does not execute a system or advance the clock.
func (w *World) ChangesSince(since Tick) SystemContext {
	requireWorld(w)
	return SystemContext{World: w, lastRun: since, thisRun: w.changeTick}
}

func (c *SystemContext) LastRun() Tick {
	if c == nil {
		return 0
	}
	return c.lastRun
}

func (c *SystemContext) ThisRun() Tick {
	if c == nil {
		return 0
	}
	return c.thisRun
}

func (context *SystemContext) Local[Tag, T any]() *T {
	if context == nil {
		panic("ecs: system context is nil")
	}
	if context.locals == nil {
		context.locals = map[reflect.Type]any{}
	}
	return cachedValue[Tag, T](context.locals)
}

type SystemConfig struct {
	System System
	Name   string
	Before []string
	After  []string
	RunIf  []Condition
}

type Schedule struct {
	name    string
	world   *World
	systems map[string]scheduledSystem
	ordered []scheduledSystem
	dirty   bool
}

type scheduledSystem struct {
	context *SystemContext
	config  SystemConfig
}

func NewSchedule(name string) *Schedule {
	return &Schedule{
		name:    name,
		systems: map[string]scheduledSystem{},
		dirty:   true,
	}
}

func (s *Schedule) Add(config SystemConfig) error {
	if s == nil {
		return fmt.Errorf("ecs: schedule is nil")
	}
	if config.Name == "" {
		return fmt.Errorf("ecs: system name is required")
	}
	if config.System == nil {
		return fmt.Errorf("ecs: system %q has nil function", config.Name)
	}
	if _, exists := s.systems[config.Name]; exists {
		return fmt.Errorf("ecs: duplicate system %q in schedule %q", config.Name, s.name)
	}
	s.systems[config.Name] = scheduledSystem{config: config, context: &SystemContext{}}
	s.dirty = true
	return nil
}

func (s *Schedule) Remove(name string) {
	if s == nil {
		return
	}
	delete(s.systems, name)
	s.dirty = true
}

func (s *Schedule) Run(world *World) error {
	if s == nil {
		return nil
	}
	if world == nil {
		return fmt.Errorf("ecs: world is nil")
	}
	if s.world == nil {
		s.world = world
	} else if s.world != world {
		return fmt.Errorf("ecs: schedule %q belongs to a different world", s.name)
	}
	if err := s.compile(); err != nil {
		return err
	}
	for _, scheduled := range s.ordered {
		ran, commandError, err := executeSystem(world, scheduled.context, scheduled.config)
		if !ran {
			continue
		}
		if err == nil {
			continue
		}
		if !commandError {
			return fmt.Errorf("ecs: schedule %q system %q failed: %w", s.name, scheduled.config.Name, err)
		}
		return fmt.Errorf("ecs: schedule %q system %q commands failed: %w", s.name, scheduled.config.Name, err)
	}
	return nil
}

func (c SystemConfig) Run(world *World) error {
	if c.System == nil {
		return fmt.Errorf("ecs: system %q has nil function", c.Name)
	}
	if world == nil {
		return fmt.Errorf("ecs: world is nil")
	}
	if world.directSystemContexts == nil {
		world.directSystemContexts = make(map[string]*SystemContext)
	}
	context := world.directSystemContexts[c.Name]
	if context == nil {
		context = &SystemContext{}
		world.directSystemContexts[c.Name] = context
	}
	_, _, err := executeSystem(world, context, c)
	return err
}

func executeSystem(world *World, context *SystemContext, config SystemConfig) (ran bool, commandError bool, err error) {
	if context.World != world {
		context.lastRun = 0
	}
	context.World = world
	context.Commands = world.Commands()
	context.thisRun = world.nextChangeTick()
	world.activeChangeTick = context.thisRun
	defer func() { world.activeChangeTick = 0 }()
	checkpoint := context.Commands.checkpoint()
	succeeded := false
	defer func() {
		if !succeeded && checkpoint <= len(context.Commands.queue) {
			context.Commands.discardFrom(checkpoint)
		}
	}()

	if !config.shouldRun(context) {
		return false, false, nil
	}
	err = config.System(context)
	if err != nil {
		return true, false, err
	}
	if err := world.FlushCommands(); err != nil {
		return true, true, err
	}
	context.lastRun = context.thisRun
	succeeded = true
	return true, false, nil
}

func (s *Schedule) Order() ([]string, error) {
	if s == nil {
		return nil, nil
	}
	if err := s.compile(); err != nil {
		return nil, err
	}
	order := make([]string, len(s.ordered))
	for i, system := range s.ordered {
		order[i] = system.config.Name
	}
	return order, nil
}

func (s *Schedule) compile() error {
	if s == nil || !s.dirty {
		return nil
	}
	scratch := borrowScheduleScratch()
	defer scratch.release()
	for name := range s.systems {
		scratch.indices[name] = len(scratch.nodes)
		scratch.nodes = append(scratch.nodes, scheduleNode{name: name, edge: -1})
	}
	for name, system := range s.systems {
		index := scratch.indices[name]
		for _, dep := range system.config.After {
			dependency, ok := scratch.indices[dep]
			if !ok {
				return fmt.Errorf("ecs: schedule %q system %q depends on missing system %q", s.name, name, dep)
			}
			scratch.addEdge(dependency, index)
		}
		for _, dep := range system.config.Before {
			dependency, ok := scratch.indices[dep]
			if !ok {
				return fmt.Errorf("ecs: schedule %q system %q references missing system %q", s.name, name, dep)
			}
			scratch.addEdge(index, dependency)
		}
	}
	for _, node := range scratch.nodes {
		if node.inDegree == 0 {
			scratch.ready = append(scratch.ready, node.name)
		}
	}
	scratch.ready.init()
	clear(s.ordered)
	s.ordered = s.ordered[:0]
	for len(scratch.ready) > 0 {
		name := scratch.ready.pop()
		s.ordered = append(s.ordered, s.systems[name])
		for edge := scratch.nodes[scratch.indices[name]].edge; edge >= 0; {
			entry := scratch.edges[edge]
			node := &scratch.nodes[entry.target]
			node.inDegree--
			if node.inDegree == 0 {
				scratch.ready.push(node.name)
			}
			edge = entry.next
		}
	}
	if len(s.ordered) != len(s.systems) {
		return fmt.Errorf("ecs: schedule %q has cyclic system dependencies", s.name)
	}
	s.dirty = false
	return nil
}

func (c SystemConfig) shouldRun(context *SystemContext) bool {
	if len(c.RunIf) == 0 {
		return true
	}
	for _, condition := range c.RunIf {
		if condition != nil && !condition(context) {
			return false
		}
	}
	return true
}
