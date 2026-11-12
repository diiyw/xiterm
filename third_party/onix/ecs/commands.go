package ecs

import (
	"fmt"
	"reflect"
	"slices"
)

type commandKind uint8

const (
	commandInsert commandKind = iota
	commandRemove
	commandDespawn
	commandSpawn
	commandRemovePrepared
)

type commandStream interface {
	apply(*World, Entity, int) error
	truncate(int)
	retain(int)
	trimRetained() bool
	setStreamID(uint32)
}

type commandValues[T any] struct {
	values   []T
	retained int
	streamID uint32
}

type typedInsertCommandStream[T any] struct {
	commandValues[T]
}

type commandValue interface {
	apply(*World, Entity) error
}

type valueCommandStream[T commandValue] struct {
	commandValues[T]
}

func (s *valueCommandStream[T]) apply(world *World, entity Entity, index int) error {
	return s.values[index].apply(world, entity)
}

func (commands *Commands) queueCommandValue[T commandValue](entity Entity, value T) {
	stream := commands.commandStreamFor(func() *valueCommandStream[T] { return &valueCommandStream[T]{} })
	stream.queue(commands, entity, value)
}

func (s *commandValues[T]) queue(commands *Commands, entity Entity, value T) {
	index := len(s.values)
	if index == 0 {
		commands.activeStreams = append(commands.activeStreams, s.streamID)
	}
	s.values = append(s.values, value)
	commands.queue = append(commands.queue, queuedCommand{kind: commandInsert, entity: entity, data: s.streamID, valueIndex: index})
}

func (s *typedInsertCommandStream[T]) apply(world *World, entity Entity, index int) error {
	if index < 0 {
		world.Remove[T](entity)
		return nil
	}
	return world.Insert(entity, s.values[index])
}

func (s *commandValues[T]) retain(length int) {
	s.retained = max(s.retained, length)
}

func (s *commandValues[T]) setStreamID(id uint32) { s.streamID = id }

func (s *commandValues[T]) trimRetained() bool {
	s.truncate(s.retained)
	s.retained = 0
	return len(s.values) != 0
}

func (s *commandValues[T]) truncate(length int) {
	if length < 0 || length > len(s.values) {
		panic("ecs: invalid command stream length")
	}
	clear(s.values[length:])
	s.values = s.values[:length]
}

type queuedCommand struct {
	entity     Entity
	valueIndex int
	// data is a stream ID, or the component count for commandSpawn. Keeping
	// references in the shared stream list makes the queue pointer-free.
	data uint32
	kind commandKind
}

type Commands struct {
	world         *World
	queue         []queuedCommand
	spawnEntries  []preparedBundleEntry
	streams       map[reflect.Type]commandStream
	streamList    []commandStream
	activeStreams []uint32 // streams with queued values; capacity is reused
	removals      []*removalPlan
}

func (c *Commands) checkpoint() int {
	if c == nil {
		panic("ecs: commands is nil")
	}
	return len(c.queue)
}

// Reserve grows command queue storage without changing its contents. It
// avoids queue backing-array growth when a system knows its command burst.
func (c *Commands) Reserve(capacity int) {
	if c == nil {
		panic("ecs: commands is nil")
	}
	if capacity > cap(c.queue) {
		c.queue = slices.Grow(c.queue, capacity-len(c.queue))
	}
}

func (c *Commands) discardFrom(index int) {
	if c == nil {
		panic("ecs: commands is nil")
	}
	if index < 0 || index > len(c.queue) {
		panic("ecs: invalid command checkpoint")
	}
	for _, queued := range c.queue[index:] {
		if queued.kind == commandSpawn {
			c.world.cancelReserved(queued.entity)
		}
	}
	clear(c.queue[index:])
	c.queue = c.queue[:index]
	c.trimStorage()
}

func (c *Commands) trimStorage() {
	if c == nil {
		panic("ecs: commands is nil")
	}
	spawnLength := 0
	removalLength := 0
	for _, queued := range c.queue {
		switch queued.kind {
		case commandInsert:
			c.streamList[queued.data].retain(queued.valueIndex + 1)
		case commandSpawn:
			spawnLength = max(spawnLength, queued.valueIndex+int(queued.data))
		case commandRemovePrepared:
			removalLength = max(removalLength, queued.valueIndex+1)
		}
	}
	active := c.activeStreams[:0]
	for _, id := range c.activeStreams {
		if c.streamList[id].trimRetained() {
			active = append(active, id)
		}
	}
	c.activeStreams = active
	clear(c.spawnEntries[spawnLength:])
	c.spawnEntries = c.spawnEntries[:spawnLength]
	clear(c.removals[removalLength:])
	c.removals = c.removals[:removalLength]
}

func (w *World) Commands() *Commands {
	requireWorld(w)
	return &w.commands
}

func (commands *Commands) Insert[T any](entity Entity, value T) {
	if commands == nil {
		panic("ecs: commands is nil")
	}
	stream := commands.insertCommandStreamFor[T]()
	stream.queue(commands, entity, value)
}

func (commands *Commands) Remove[T any](entity Entity) {
	if commands == nil {
		panic("ecs: commands is nil")
	}
	stream := commands.insertCommandStreamFor[T]()
	commands.queue = append(commands.queue, queuedCommand{
		kind:   commandRemove,
		entity: entity,
		data:   stream.streamID,
	})
}

// RemovePrepared queues one batch migration for use during iteration.
func (commands *Commands) RemovePrepared(entity Entity, removal PreparedRemoval) {
	if commands == nil {
		panic("ecs: commands is nil")
	}
	plan := removal.requirePlan()
	if plan.world != commands.world {
		panic("ecs: prepared removal belongs to a different world")
	}
	index := len(commands.removals)
	commands.removals = append(commands.removals, plan)
	commands.queue = append(commands.queue, queuedCommand{kind: commandRemovePrepared, entity: entity, valueIndex: index})
}

func (commands *Commands) Despawn(entity Entity) {
	if commands == nil {
		panic("ecs: commands is nil")
	}
	commands.queue = append(commands.queue, queuedCommand{kind: commandDespawn, entity: entity})
}

func (commands *Commands) Spawn(bundle Bundle) Entity {
	if commands == nil {
		panic("ecs: commands is nil")
	}
	if commands.world == nil {
		panic("ecs: commands is not attached to a world")
	}
	start := len(commands.spawnEntries)
	defer func() {
		if value := recover(); value != nil {
			clear(commands.spawnEntries[start:])
			commands.spawnEntries = commands.spawnEntries[:start]
			panic(value)
		}
	}()
	commands.spawnEntries = slices.Grow(commands.spawnEntries, len(bundle.entries))
	commands.spawnEntries = commands.spawnEntries[:start+len(bundle.entries)]
	entries := commands.world.prepareBundle(bundle, commands.spawnEntries[start:])
	if uint64(len(entries)) > uint64(^uint32(0)) {
		panic("ecs: command spawn component capacity exhausted")
	}
	entity := commands.world.reserveEntity()
	commands.queue = append(commands.queue, queuedCommand{
		kind:       commandSpawn,
		entity:     entity,
		valueIndex: start,
		data:       uint32(len(entries)),
	})
	return entity
}

// SpawnPrepared reserves an entity and queues a previously prepared
// bundle without rebuilding its component entry list.
func (commands *Commands) SpawnPrepared(bundle PreparedBundle) Entity {
	if commands == nil {
		panic("ecs: commands is nil")
	}
	if commands.world == nil {
		panic("ecs: commands is not attached to a world")
	}
	if bundle.world != commands.world {
		panic("ecs: prepared bundle belongs to a different world")
	}
	if uint64(len(bundle.entries)) > uint64(^uint32(0)) {
		panic("ecs: command spawn component capacity exhausted")
	}
	start := len(commands.spawnEntries)
	defer func() {
		if value := recover(); value != nil {
			clear(commands.spawnEntries[start:])
			commands.spawnEntries = commands.spawnEntries[:start]
			panic(value)
		}
	}()
	commands.spawnEntries = append(commands.spawnEntries, bundle.entries...)
	entity := commands.world.reserveEntity()
	commands.queue = append(commands.queue, queuedCommand{
		kind:       commandSpawn,
		entity:     entity,
		valueIndex: start,
		data:       uint32(len(bundle.entries)),
	})
	return entity
}

func (commands *Commands) insertCommandStreamFor[T any]() *typedInsertCommandStream[T] {
	return commands.commandStreamFor(func() *typedInsertCommandStream[T] { return &typedInsertCommandStream[T]{} })
}

func (commands *Commands) commandStreamFor[S commandStream](create func() S) S {
	if commands.streams == nil {
		commands.streams = make(map[reflect.Type]commandStream)
	}
	typ := reflect.TypeFor[S]()
	if stream, ok := commands.streams[typ]; ok {
		return stream.(S)
	}
	stream := create()
	if uint64(len(commands.streamList)) > uint64(^uint32(0)) {
		panic("ecs: command stream capacity exhausted")
	}
	stream.setStreamID(uint32(len(commands.streamList)))
	commands.streamList = append(commands.streamList, stream)
	commands.streams[typ] = stream
	return stream
}

func (c queuedCommand) apply(world *World, spawnEntries []preparedBundleEntry) error {
	switch c.kind {
	case commandInsert:
		return world.commands.streamList[c.data].apply(world, c.entity, c.valueIndex)
	case commandRemove:
		return world.commands.streamList[c.data].apply(world, c.entity, -1)
	case commandDespawn:
		world.Despawn(c.entity)
		return nil
	case commandSpawn:
		world.spawnReserved(c.entity, spawnEntries[c.valueIndex:c.valueIndex+int(c.data)], world.mutationTick())
		return nil
	case commandRemovePrepared:
		PreparedRemoval{plan: world.commands.removals[c.valueIndex]}.Apply(c.entity)
		return nil
	default:
		panic("ecs: unknown command kind")
	}
}

func (w *World) FlushCommands() error {
	requireWorld(w)
	w.assertStructuralMutationAllowed()
	queue := w.commands.queue
	if len(queue) == 0 {
		return nil
	}
	spawnEntries := w.commands.spawnEntries
	w.commands.queue = nil
	w.commands.spawnEntries = nil
	next := 0
	defer func() {
		clear(queue)
		w.commands.queue = queue[:0]
		w.commands.spawnEntries = spawnEntries
		w.commands.resetStorage()
	}()
	defer func() {
		if value := recover(); value != nil {
			w.commands.queue = queue[next:]
			w.commands.discardFrom(0)
			panic(value)
		}
	}()
	for index, queued := range queue {
		next = index
		if err := queued.apply(w, spawnEntries); err != nil {
			w.commands.queue = queue[index+1:]
			w.commands.discardFrom(0)
			return fmt.Errorf("ecs: apply command %d: %w", index, err)
		}
		next = index + 1
	}
	return nil
}

func (c *Commands) resetStorage() {
	if c == nil {
		panic("ecs: commands is nil")
	}
	for _, id := range c.activeStreams {
		c.streamList[id].truncate(0)
	}
	c.activeStreams = c.activeStreams[:0]
	clear(c.spawnEntries)
	c.spawnEntries = c.spawnEntries[:0]
	clear(c.removals)
	c.removals = c.removals[:0]
}
