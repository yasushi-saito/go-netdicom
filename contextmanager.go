package netdicom

import (
	"fmt"
	"github.com/yasushi-saito/go-dicom"
	"log"
)

type contextManagerEntry struct {
	contextID         byte
	abstractSyntaxUID string
	transferSyntaxUID string
}

// contextManager manages mappings between a contextID and the corresponding
// abstract-syntax UID (aka SOP).  UID is of form "1.2.840.10008.5.1.4.1.1.1.2".
// UIDs are static and global. They are defined in
// https://www.dicomlibrary.com/dicom/sop/.
//
// On the other hand, contextID is allocated anew during each association
// handshake.  ContextID values are 1, 3, 5, etc.  One contextManager is created
// per association.
type contextManager struct {
	// The two maps are inverses of each other.
	contextIDToAbstractSyntaxNameMap map[byte]*contextManagerEntry
	abstractSyntaxNameToContextIDMap map[string]*contextManagerEntry

	// tmpRequests used only on the client (requestor) side. It holds the
	// contextid->presentationcontext mapping generated from the
	// A_ASSOCIATE_RQ PDU. Once an A_ASSOCIATE_AC PDU arrives, tmpRequests
	// is matched against the response PDU and
	// contextid->{abstractsyntax,transfersyntax} mappings are filled.
	tmpRequests map[byte]*PresentationContextItem
}

// Create an empty contextManager
func newContextManager() *contextManager {
	c := &contextManager{
		contextIDToAbstractSyntaxNameMap: make(map[byte]*contextManagerEntry),
		abstractSyntaxNameToContextIDMap: make(map[string]*contextManagerEntry),
		tmpRequests:                      make(map[byte]*PresentationContextItem),
	}
	log.Printf("new context manager(%p)", c)
	return c
}

// Called by the user (client) to produce an A_ASSOCIATE_RQ items.
func (m *contextManager) generateAssociateRequest(
	services []SOPUID, transferSyntaxUIDs []string) []*PresentationContextItem {

	var items []*PresentationContextItem
	var contextID byte = 1
	for _, sop := range services {
		syntaxItems := []SubItem{
			&AbstractSyntaxSubItem{Name: sop.UID},
		}
		for _, syntaxUID := range transferSyntaxUIDs {
			syntaxItems = append(syntaxItems, &TransferSyntaxSubItem{Name: syntaxUID})
		}
		item := &PresentationContextItem{
			Type:      ItemTypePresentationContextRequest,
			ContextID: contextID,
			Result:    0, // must be zero for request
			Items:     syntaxItems,
		}
		items = append(items, item)
		m.tmpRequests[contextID] = item
		contextID += 2 // must be odd.
	}
	return items
}

// Called when A_ASSOCIATE_RQ pdu arrives, on the provider side. Returns a list of items to be sent in
// the A_ASSOCIATE_AC pdu.
func (m *contextManager) onAssociateRequest(requests []*PresentationContextItem) ([]*PresentationContextItem, error) {
	var responses []*PresentationContextItem
	for _, contextItem := range requests {
		var sopUID string
		var pickedTransferSyntaxUID string
		for _, subItem := range contextItem.Items {
			switch c := subItem.(type) {
			case *AbstractSyntaxSubItem:
				if sopUID != "" {
					log.Fatalf("Multiple AbstractSyntaxSubItem found in %v", contextItem.String())
				}
				sopUID = c.Name
			case *TransferSyntaxSubItem:
				// Just pick the first syntax UID proposed by the client.
				if pickedTransferSyntaxUID == "" {
					pickedTransferSyntaxUID = c.Name
				}
			default:
				return nil, fmt.Errorf("Unknown subitem in PresentationContext: %s", subItem.String())
			}
		}
		if sopUID == "" || pickedTransferSyntaxUID == "" {
			log.Fatalf("SOP or transfersyntax not found in PresentationContext: %v", contextItem.String())
		}
		responses = append(responses, &PresentationContextItem{
			Type:      ItemTypePresentationContextResponse,
			ContextID: contextItem.ContextID,
			Result:    0, // accepted
			Items:     []SubItem{&TransferSyntaxSubItem{Name: pickedTransferSyntaxUID}}})
		log.Printf("Provider(%p): addmapping %v %v %v", m, sopUID, pickedTransferSyntaxUID, contextItem.ContextID)
		addContextMapping(m, sopUID, pickedTransferSyntaxUID, contextItem.ContextID)
	}
	return responses, nil
}

// Called by the user (client) to when A_ASSOCIATE_AC PDU arrives from the provider.
func (m *contextManager) onAssociateResponse(responses []*PresentationContextItem) error {
	for _, contextItem := range responses {
		var pickedTransferSyntaxUID string
		for _, subItem := range contextItem.Items {
			switch c := subItem.(type) {
			case *TransferSyntaxSubItem:
				// Just pick the first syntax UID proposed by the client.
				if pickedTransferSyntaxUID == "" {
					pickedTransferSyntaxUID = c.Name
				} else {
					return fmt.Errorf("Multiple syntax UIDs returned in A_ASSOCIATE_AC: %v", contextItem.String())
				}
			default:
				return fmt.Errorf("Unknown subitem %s in PresentationContext: %s", subItem.String(), contextItem.String())
			}
		}
		request, ok := m.tmpRequests[contextItem.ContextID]
		if !ok {
			return fmt.Errorf("Unknown context ID %d for A_ASSOCIATE_AC: %v",
				contextItem.ContextID,
				contextItem.String())
		}
		found := false
		var sopUID string
		for _, subItem := range request.Items {
			switch c := subItem.(type) {
			case *AbstractSyntaxSubItem:
				sopUID = c.Name
			case *TransferSyntaxSubItem:
				if c.Name == pickedTransferSyntaxUID {
					found = true
					break
				}
			}
		}
		if !found || sopUID == "" {
			return fmt.Errorf("TransferSyntaxUID or AbstractSyntaxSubItem not found in %v", contextItem.String())
		}
		addContextMapping(m, sopUID, pickedTransferSyntaxUID, contextItem.ContextID)
	}
	return nil
}

// Add a mapping between a (global) UID and a (per-session) context ID.
func addContextMapping(
	m *contextManager,
	abstractSyntaxUID string,
	transferSyntaxUID string,
	contextID byte) {
	log.Printf("Map context %d -> %s, %s",
		contextID, dicom.UIDString(abstractSyntaxUID),
		dicom.UIDString(transferSyntaxUID))
	doassert(abstractSyntaxUID != "")
	doassert(transferSyntaxUID != "")
	doassert(contextID%2 == 1)
	e := &contextManagerEntry{
		abstractSyntaxUID: abstractSyntaxUID,
		transferSyntaxUID: transferSyntaxUID,
		contextID:         contextID,
	}
	m.contextIDToAbstractSyntaxNameMap[contextID] = e
	m.abstractSyntaxNameToContextIDMap[abstractSyntaxUID] = e
}

// Convert an UID to a context ID.
func (m *contextManager) lookupByAbstractSyntaxUID(name string) (contextManagerEntry, error) {
	e, ok := m.abstractSyntaxNameToContextIDMap[name]
	if !ok {
		return contextManagerEntry{}, fmt.Errorf("contextmanager(%p): Unknown syntax %s", name)
	}
	return *e, nil
}

// Convert a contextID to a UID.
func (m *contextManager) lookupByContextID(contextID byte) (contextManagerEntry, error) {
	e, ok := m.contextIDToAbstractSyntaxNameMap[contextID]
	if !ok {
		return contextManagerEntry{}, fmt.Errorf("contextmanager(%p): Unknown context ID %d", m, contextID)
	}
	return *e, nil
}
