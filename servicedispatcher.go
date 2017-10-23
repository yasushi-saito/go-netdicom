package netdicom

import (
	"fmt"
	"sync"

	"github.com/yasushi-saito/go-netdicom/dimse"
	"v.io/x/lib/vlog"
)

type serviceCommandState struct {
	disp      *serviceDispatcher  // Parent.
	messageID uint16              // Provider MessageID.
	context   contextManagerEntry // Transfersyntax/sopclass for this command.
	cm        *contextManager     // For looking up context -> transfersyntax/sopclass mappings

	// upcallCh streams PROVIDER command+data for the given messageID.
	upcallCh chan upcallEvent
}

func (cs *serviceCommandState) sendMessage(resp dimse.Message, data []byte) {
	vlog.VI(1).Infof("Sending PROVIDER message: %v %v", resp, cs.disp)
	payload := &stateEventDIMSEPayload{
		abstractSyntaxName: cs.context.abstractSyntaxUID,
		command:            resp,
		data:               data,
	}
	cs.disp.downcallCh <- stateEvent{
		event:        evt09,
		pdu:          nil,
		conn:         nil,
		dimsePayload: payload,
	}
}

type serviceCallback func(
	msg dimse.Message, data []byte,
	cs *serviceCommandState)

type serviceDispatcher struct {
	downcallCh chan stateEvent // for sending PDUs to the statemachine.

	mu             sync.Mutex
	activeCommands map[uint16]*serviceCommandState // guarded by mu
	callbacks      map[int]serviceCallback         // guarded by mu
}

func (disp *serviceDispatcher) findOrCreateCommand(
	messageID uint16,
	cm *contextManager,
	context contextManagerEntry) (*serviceCommandState, bool) {
	disp.mu.Lock()
	defer disp.mu.Unlock()
	if cs, ok := disp.activeCommands[messageID]; ok {
		return cs, true
	}
	cs := &serviceCommandState{
		disp:      disp,
		messageID: messageID,
		cm:        cm,
		context:   context,
		upcallCh:  make(chan upcallEvent, 128),
	}
	disp.activeCommands[messageID] = cs
	vlog.VI(1).Infof("Start provider command %v", messageID)
	return cs, false
}

func (disp *serviceDispatcher) deleteCommand(cs *serviceCommandState) {
	disp.mu.Lock()
	vlog.VI(1).Infof("Finish provider command %v", cs.messageID)
	if _, ok := disp.activeCommands[cs.messageID]; !ok {
		panic(fmt.Sprintf("cs %+v", cs))
	}
	delete(disp.activeCommands, cs.messageID)
	disp.mu.Unlock()
}

func (disp *serviceDispatcher) registerCallback(commandField int, cb serviceCallback) {
	disp.mu.Lock()
	disp.callbacks[commandField] = cb
	disp.mu.Unlock()
}

func (disp *serviceDispatcher) unregisterCallback(commandField int) {
	disp.mu.Lock()
	delete(disp.callbacks, commandField)
	disp.mu.Unlock()
}

func (disp *serviceDispatcher) handleEvent(event upcallEvent) {
	if event.eventType == upcallEventHandshakeCompleted {
		return
	}
	doassert(event.eventType == upcallEventData)
	doassert(event.command != nil)
	context, err := event.cm.lookupByContextID(event.contextID)
	if err != nil {
		vlog.Infof("Invalid context ID %d: %v", event.contextID, err)
		disp.downcallCh <- stateEvent{event: evt19, pdu: nil, err: err}
		return
	}
	messageID := event.command.GetMessageID()
	dc, found := disp.findOrCreateCommand(messageID, event.cm, context)
	if found {
		vlog.VI(1).Infof("Forwarding command to existing command: %+v", event.command, dc)
		dc.upcallCh <- event
		vlog.VI(1).Infof("Done forwarding command to existing command: %+v", event.command, dc)
		return
	}
	disp.mu.Lock()
	cb := disp.callbacks[event.command.CommandField()]
	disp.mu.Unlock()
	go func() {
		cb(event.command, event.data, dc)
		disp.deleteCommand(dc)
	}()
}

func (disp *serviceDispatcher) close() {
	disp.mu.Lock()
	for _, cs := range disp.activeCommands {
		close(cs.upcallCh)
	}
	disp.mu.Unlock()
	// TODO(saito): prevent new command from launching.
}

func newServiceDispatcher() *serviceDispatcher {
	return &serviceDispatcher{
		downcallCh:     make(chan stateEvent, 128),
		activeCommands: make(map[uint16]*serviceCommandState),
		callbacks:      make(map[int]serviceCallback),
	}
}
