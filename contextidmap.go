package netdicom

import (
	"fmt"
)

type contextIDMapEntry struct {
	contextID         byte
	abstractSyntaxUID string
	transferSyntaxUID string
}

// contextIDMap manages mappings between a contextID and the corresponding
// abstract-syntax UID (aka SOP).  UID is of form "1.2.840.10008.5.1.4.1.1.1.2".
// UIDs are static and global. They are defined in
// https://www.dicomlibrary.com/dicom/sop/.
//
// On the other hand, contextID is allocated anew during each association
// handshake.  ContextID values are 1, 3, 5, etc.  One contextIDMap is created
// per association.
type contextIDMap struct {
	// The two maps are inverses of each other.
	contextIDToAbstractSyntaxNameMap map[byte]*contextIDMapEntry
	abstractSyntaxNameToContextIDMap map[string]*contextIDMapEntry
}

// Create an empty contextIDMap
func newContextIDMap() *contextIDMap {
	return &contextIDMap{
		contextIDToAbstractSyntaxNameMap: make(map[byte]*contextIDMapEntry),
		abstractSyntaxNameToContextIDMap: make(map[string]*contextIDMapEntry),
	}
}

// Add a mapping between a (global) UID and a (per-session) context ID.
func (m *contextIDMap) addMapping(
	abstractSyntaxUID string,
	transferSyntaxUID string,
	contextID byte) {
	// doassert(dicom.MustLookupUID(abstractSyntaxUID).Type == dicom.UIDTypeSOPClass)
	// TODO(saito) This assertion doesn't hold because the client side passes a bogus uid.
	// doassert(dicom.MustLookupUID(transferSyntaxUID).Type == dicom.UIDTypeTransferSyntax)
	doassert(contextID%2 == 1)
	e := &contextIDMapEntry{
		abstractSyntaxUID: abstractSyntaxUID,
		transferSyntaxUID: transferSyntaxUID,
		contextID:         contextID,
	}
	m.contextIDToAbstractSyntaxNameMap[contextID] = e
	m.abstractSyntaxNameToContextIDMap[abstractSyntaxUID] = e
}

// Convert an UID to a context ID.
func (m *contextIDMap) lookupByAbstractSyntaxUID(name string) (contextIDMapEntry, error) {
	e, ok := m.abstractSyntaxNameToContextIDMap[name]
	if !ok {
		return contextIDMapEntry{}, fmt.Errorf("Unknown syntax %s", name)
	}
	return *e, nil
}

// Convert a contextID to a UID.
func (m *contextIDMap) lookupByContextID(contextID byte) (contextIDMapEntry, error) {
	e, ok := m.contextIDToAbstractSyntaxNameMap[contextID]
	if !ok {
		return contextIDMapEntry{}, fmt.Errorf("Unknown context ID %d", contextID)
	}
	return *e, nil
}
