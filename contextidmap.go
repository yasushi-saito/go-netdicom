package netdicom

import (
	"fmt"
)

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
	contextIDToAbstractSyntaxNameMap map[byte]string
	abstractSyntaxNameToContextIDMap map[string]byte
}

// Create an empty contextIDMap
func newContextIDMap() *contextIDMap {
	return &contextIDMap{
		contextIDToAbstractSyntaxNameMap: make(map[byte]string),
		abstractSyntaxNameToContextIDMap: make(map[string]byte),
	}
}

// Add a mapping between a (global) UID and a (per-session) context ID.
func addContextIDToAbstractSyntaxNameMap(m *contextIDMap, name string, contextID byte) {
	m.contextIDToAbstractSyntaxNameMap[contextID] = name
	m.abstractSyntaxNameToContextIDMap[name] = contextID
}

// Convert an UID to a context ID.
func abstractSyntaxNameToContextID(m *contextIDMap, name string) (byte, error) {
	id, ok := m.abstractSyntaxNameToContextIDMap[name]
	if !ok {
		return 0, fmt.Errorf("Unknown syntax %s", name)
	}
	return id, nil
}

// Convert a contextID to a UID.
func contextIDToAbstractSyntaxName(m *contextIDMap, contextID byte) (string, error) {
	name, ok := m.contextIDToAbstractSyntaxNameMap[contextID]
	if !ok {
		return "", fmt.Errorf("Unknown context ID %d", contextID)
	}
	return name, nil
}
