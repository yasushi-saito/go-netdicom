#!/usr/bin/env python3.6

import enum
from typing import IO, List, NamedTuple

Field = NamedTuple('Field', [('name', str),
                             ('type', str),
                             ('required', bool)])
class Type(enum.Enum):
    REQUEST = 1
    RESPONSE = 2

Message = NamedTuple('Message',
                     [('name', str),
                      ('type', Type),
                      ('command_field', int),
                      ('fields', List[Field])])

# class Field(object):
#     def __init__(name: str, tag, typename: str, required: bool):
#         self.name = name
#         self.typename = typename
#         self.required = required

MESSAGES = [
    # P3.7 9.3.1.1
    Message('C_STORE_RQ',
            Type.REQUEST, 1,
            [Field('AffectedSOPClassUID', 'string', True),
             Field('MessageID', 'uint16', True),
             Field('Priority', 'uint16', True),
             Field('CommandDataSetType', 'uint16', True),
             Field('AffectedSOPInstanceUID', 'string', True),
	     Field('MoveOriginatorApplicationEntityTitle', 'string', False),
	     Field('MoveOriginatorMessageID', 'uint16', False)]),
    # P3.7 9.3.1.2
    Message('C_STORE_RSP',
            Type.RESPONSE, 0x8001,
            [Field('AffectedSOPClassUID', 'string', True),
             Field('MessageIDBeingRespondedTo', 'uint16', True),
             Field('CommandDataSetType', 'uint16', True),
             Field('AffectedSOPInstanceUID', 'string', True),
	     Field('Status', 'Status', True)]),
    # P3.7 9.1.2.1
    Message('C_FIND_RQ',
            Type.REQUEST, 0x20,
            [Field('AffectedSOPClassUID', 'string', True),
             Field('MessageID', 'uint16', True),
             Field('Priority', 'uint16', True),
             Field('CommandDataSetType', 'uint16', True)]),
    Message('C_FIND_RSP',
            Type.RESPONSE, 0x8020,
            [Field('AffectedSOPClassUID', 'string', True),
             Field('MessageIDBeingRespondedTo', 'uint16', True),
             Field('CommandDataSetType', 'uint16', True),
	     Field('Status', 'Status', True)]),
    # P3.7 9.1.2.1
    Message('C_GET_RQ',
            Type.REQUEST, 0x10,
            [Field('AffectedSOPClassUID', 'string', True),
             Field('MessageID', 'uint16', True),
             Field('Priority', 'uint16', True),
             Field('CommandDataSetType', 'uint16', True)]),
    Message('C_GET_RSP',
            Type.RESPONSE, 0x8010,
            [Field('AffectedSOPClassUID', 'string', True),
             Field('MessageIDBeingRespondedTo', 'uint16', True),
             Field('CommandDataSetType', 'uint16', True),
             Field('NumberOfRemainingSuboperations', 'uint16', False),
             Field('NumberOfCompletedSuboperations', 'uint16', False),
             Field('NumberOfFailedSuboperations', 'uint16', False),
             Field('NumberOfWarningSuboperations', 'uint16', False),
	     Field('Status', 'Status', True)]),
    # P3.7 9.3.4.1
    Message('C_MOVE_RQ',
            Type.REQUEST, 0x21,
            [Field('AffectedSOPClassUID', 'string', True),
             Field('MessageID', 'uint16', True),
             Field('Priority', 'uint16', True),
             Field('MoveDestination', 'string', True),
             Field('CommandDataSetType', 'uint16', True)]),
    Message('C_MOVE_RSP',
            Type.RESPONSE, 0x8021,
            [Field('AffectedSOPClassUID', 'string', True),
             Field('MessageIDBeingRespondedTo', 'uint16', True),
             Field('CommandDataSetType', 'uint16', True),
             Field('NumberOfRemainingSuboperations', 'uint16', False),
             Field('NumberOfCompletedSuboperations', 'uint16', False),
             Field('NumberOfFailedSuboperations', 'uint16', False),
             Field('NumberOfWarningSuboperations', 'uint16', False),
	     Field('Status', 'Status', True)]),
    # P3.7 9.3.5
    Message('C_ECHO_RQ',
            Type.REQUEST, 0x30,
            [Field('MessageID', 'uint16', True),
             Field('CommandDataSetType', 'uint16', True)]),
    Message('C_ECHO_RSP',
            Type.RESPONSE, 0x8030,
            [Field('MessageIDBeingRespondedTo', 'uint16', True),
             Field('CommandDataSetType', 'uint16', True),
	     Field('Status', 'Status', True)])
]

def generate_go_definition(m: Message, out: IO[str]):
    print(f'type {m.name} struct  {{', file=out)
    for f in m.fields:
        print(f'	{f.name} {f.type}', file=out)
    print(f'	Extra []*dicom.Element  // Unparsed elements', file=out)
    print('}', file=out)

    print('', file=out)
    print(f'func (v* {m.name}) Encode(e *dicomio.Encoder) {{', file=out)
    print(f'	encodeField(e, dicom.TagCommandField, uint16({m.command_field}))', file=out)
    for f in m.fields:
        if not f.required:
            if f.type == 'string':
                zero = '""'
            else:
                zero = '0'
            print(f'	if v.{f.name} != {zero} {{', file=out)
            print(f'		encodeField(e, dicom.Tag{f.name}, v.{f.name})', file=out)
            print(f'	}}', file=out)
        elif f.type == 'Status':
            print(f'	encodeStatus(e, v.{f.name})', file=out)
        else:
            print(f'	encodeField(e, dicom.Tag{f.name}, v.{f.name})', file=out)
    print('	for _, elem := range v.Extra {', file=out)
    print('		dicom.WriteElement(e, elem)', file=out)
    print('	}', file=out)
    print('}', file=out)

    print('', file=out)
    print(f'func (v* {m.name}) HasData() bool {{', file=out)
    print(f'	return v.CommandDataSetType != CommandDataSetTypeNull', file=out)
    print('}', file=out)

    print('', file=out)
    print(f'func (v* {m.name}) String() string {{', file=out)
    i = 0
    fmt = f'{m.name}{{'
    args = ''
    for f in m.fields:
        space = ''
        if i > 0:
            fmt += ' '
            args += ', '
        fmt += f'{f.name}:%v'
        args += f'v.{f.name}'
        i += 1
    print(f'	return fmt.Sprintf("{fmt}", {args})', file=out)
    print('}', file=out)


    print('', file=out)
    print(f'func decode{m.name}(d *messageDecoder) *{m.name} {{', file=out)
    print(f'	v := &{m.name}{{}}', file=out)
    for f in m.fields:
        if f.type == 'Status':
            print(f'	v.{f.name} = d.getStatus()', file=out)
        else:
            if f.type == 'string':
                decoder = 'String'
            elif f.type == 'uint16':
                decoder = 'UInt16'
            elif f.type == 'uint32':
                decoder = 'UInt32'
            else:
                raise Exception(f)
            if f.required:
                required = 'RequiredElement'
            else:
                required = 'OptionalElement'
            print(f'	v.{f.name} = d.get{decoder}(dicom.Tag{f.name}, {required})', file=out)
    print(f'	v.Extra = d.unparsedElements()', file=out)
    print(f'	return v', file=out)
    print('}', file=out)

def main():
    with open('dimse_messages.go', 'w') as out:
        print("""
// Auto-generated from generate_dimse_messages.py. DO NOT EDIT.
package dimse
import (
	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-dicom/dicomio"
        "fmt"
)
        """, file=out)
        for m in MESSAGES:
            generate_go_definition(m, out)

        print('func decodeMessageForType(d* messageDecoder, commandField uint16) Message {', file=out)
        print('	switch commandField {', file=out)
        for m in MESSAGES:
            print('	case 0x%x:' % (m.command_field, ), file=out)
            print(f'		return decode{m.name}(d)', file=out)
        print('	default:', file=out)
        print('		d.setError(fmt.Errorf("Unknown DIMSE command 0x%x", commandField))', file=out)
        print('		return nil', file=out)
        print('	}', file=out)
        print('}', file=out)

main()
