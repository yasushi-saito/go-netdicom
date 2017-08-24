#!/usr/bin/env python3.6

class MessageDef(object):
    def __init__(name, fields):
        self.name = name
        self.fields = fields

class Field(object)
    def __init__(name: str, tag, typename: str, required: bool):
        self.name = name
        self.tag = tag
        self.typename = typename
        self.required = required

MESSAGES = [
    MessageDef('C_STORE_RQ',
               [Field('MessageID', (0, 0x110), 'uint16', true),
                Field('Priority', (0, 0x700), 'uint16', true),
                Field('CommandDataSetType', (0, 0x800), 'uint16'),
                Field('AffectedSOPInstanceUID', (0, 0x1000), 'string'),
	        Field('MoveOriginatorApplicationEntityTitle',(0000,0x1030), 'string', False),
	        Field('MoveOriginatorMessageID', (0000,0x1031), 'uint16', False)])]

def generate_go_definition(m: MessageDef):
    print(f"type {m.name} struct  {")
    print("        Header DIMSEMessageHeader")
    for f in m.fields:
        print(f"        {f.name} {f.type}")
    print("}")

    print(f"func (v* {m.name}) Encode(e *dicom.Encoder) {")
    print("        encodeDIMSEMessageHeader(e, v.Header)")


def main():
    for m in MESSAGES:
        generate_go_definition(m)

main()
