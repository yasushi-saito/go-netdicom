#!/usr/bin/env python3.6

from typing import IO, List, NamedTuple

Field = NamedTuple('Field', [('name', str),
                             ('type', str),
                             ('required', bool)])
Message = NamedTuple('Message',
                     [('name', str),
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
            1,
            [Field('AffectedSOPClassUID', 'string', True),
             Field('MessageID', 'uint16', True),
             Field('Priority', 'uint16', True),
             Field('CommandDataSetType', 'uint16', True),
             Field('AffectedSOPInstanceUID', 'string', True),
	     Field('MoveOriginatorApplicationEntityTitle', 'string', False),
	     Field('MoveOriginatorMessageID', 'uint16', False)]),
    # P3.7 9.3.1.2
    Message('C_STORE_RSP',
            0x8001,
            [Field('AffectedSOPClassUID', 'string', True),
             Field('MessageIDBeingRespondedTo', 'uint16', True),
             Field('CommandDataSetType', 'uint16', True),
             Field('AffectedSOPInstanceUID', 'string', True),
	     Field('Status', 'uint16', True)]),
    # P3.7 9.3.5
    Message('C_ECHO_RQ',
            0x30,
            [Field('MessageID', 'uint16', True),
             Field('CommandDataSetType', 'uint16', True)]),
    Message('C_ECHO_RSP',
            0x8030,
            [Field('MessageIDBeingRespondedTo', 'uint16', True),
             Field('CommandDataSetType', 'uint16', True),
	     Field('Status', 'uint16', True)])
]

def generate_go_definition(m: Message, out: IO[str]):
    print(f'type {m.name} struct  {{', file=out)
    for f in m.fields:
        print(f'	{f.name} {f.type}', file=out)
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
        else:
            print(f'	encodeField(e, dicom.Tag{f.name}, v.{f.name})', file=out)
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
    print(f'func decode{m.name}(d *dimseDecoder) *{m.name} {{', file=out)
    print(f'	v := &{m.name}{{}}', file=out)
    for f in m.fields:
        if f.type == 'string':
            decoder = 'String'
        elif f.type == 'uint16':
            decoder = 'UInt16'
        elif f.type == 'uint32':
            decoder = 'UInt32'
        else:
            raise Exception(f, file=out)
        if f.required:
            required = 'RequiredElement'
        else:
            required = 'OptionalElement'
        print(f'	v.{f.name} = d.get{decoder}(dicom.Tag{f.name}, {required})', file=out)
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

        print('func decodeMessageForType(d* dimseDecoder, commandField uint16) DIMSEMessage {', file=out)
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
