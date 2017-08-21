package netdicom

import (
	"fmt"
)

type contextIDMap struct {
	// abstractSyntaxMap maps a contextID (an odd integer) to an abstract
	// syntax string such as 1.2.840.10008.5.1.4.1.1.1.2.  This field is set
	// on receiving A_ASSOCIATE_RQ message. Thus, it is set only on the
	// provider side (not the user).
	contextIDToAbstractSyntaxNameMap map[byte]string
	abstractSyntaxNameToContextIDMap map[string]byte
}

func newContextIDMap() *contextIDMap {
	return &contextIDMap{
		contextIDToAbstractSyntaxNameMap: make(map[byte]string),
		abstractSyntaxNameToContextIDMap: make(map[string]byte),
	}
}

func addContextIDToAbstractSyntaxNameMap(m *contextIDMap, name string, contextID byte) {
	m.contextIDToAbstractSyntaxNameMap[contextID] = name
	m.abstractSyntaxNameToContextIDMap[name] = contextID
}

func abstractSyntaxNameToContextID(m *contextIDMap, name string) (byte, error) {
	id, ok := m.abstractSyntaxNameToContextIDMap[name]
	if !ok {
		return 0, fmt.Errorf("Unknown syntax %s", name)
	}
	return id, nil
}

func contextIDToAbstractSyntaxName(m *contextIDMap, contextID byte) (string, error) {
	name, ok := m.contextIDToAbstractSyntaxNameMap[contextID]
	if !ok {
		return "", fmt.Errorf("Unknown context ID %d", contextID)
	}
	return name, nil
}
