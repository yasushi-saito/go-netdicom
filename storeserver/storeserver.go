package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-dicom/dicomio"
	"github.com/yasushi-saito/go-netdicom"
	"github.com/yasushi-saito/go-netdicom/dimse"
	"io/ioutil"
	"path"
	"strings"
	"sync/atomic"
	"v.io/x/lib/vlog"
)

var (
	portFlag   = flag.String("port", "10000", "TCP port to listen to")
	outputFlag = flag.String("output", ".", "The directory to store incoming files")
)

var pathSeq int32

func onCEchoRequest() dimse.Status {
	vlog.Info("Received C-ECHO")
	return dimse.Success
}

func onCStoreRequest(
	transferSyntaxUID string,
	sopClassUID string,
	sopInstanceUID string,
	data []byte) dimse.Status {
	path := path.Join(*outputFlag, fmt.Sprintf("image%04d.dcm", atomic.AddInt32(&pathSeq, 1)))

	vlog.Infof("Writing %s", path)
	e := dicomio.NewEncoder(binary.LittleEndian, dicomio.ExplicitVR)
	dicom.WriteFileHeader(e, transferSyntaxUID, sopClassUID, sopInstanceUID)
	e.WriteBytes(data)
	bytes, err := e.Finish()

	if err != nil {
		vlog.Errorf("%s: failed to write: %v", path, err)
		return dimse.Status{Status: dimse.StatusNotAuthorized}
	}
	err = ioutil.WriteFile(path, bytes, 0644)
	if err != nil {
		vlog.Errorf("%s: %s", path, err)
		return dimse.Status{Status: dimse.StatusNotAuthorized}
	}
	return dimse.Success
}

func onCFindRequest(transferSyntaxUID string,
	sopClassUID string,
	data []byte) dimse.Status {
	decoder := dicomio.NewBytesDecoder(data, nil, dicomio.UnknownVR)
	endian, implicit, err := dicom.ParseTransferSyntaxUID(transferSyntaxUID)
	if err != nil {
		vlog.Errorf("CFIND: Invalid transfer syntax specified by the client: %v", err)
		return dimse.Status{Status: dimse.CStoreStatusOutOfResources}
	}
	decoder.PushTransferSyntax(endian, implicit)
	var elems []*dicom.Element
	for decoder.Len() > 0 {
		elem := dicom.ReadDataElement(decoder)
		if decoder.Error() != nil {
			break
		}
		vlog.Infof("CFind param: %v", elem)
		elems = append(elems, elem)
	}
	if decoder.Error() != nil {
		return dimse.Status{
			Status:       dimse.CFindUnableToProcess,
			ErrorComment: decoder.Error().Error(),
		}
	}
	return dimse.Success
}

func main() {
	flag.Parse()
	vlog.ConfigureLibraryLoggerFromFlags()
	port := *portFlag
	if !strings.Contains(port, ":") {
		port = ":" + port
	}
	vlog.Infof("Listening on %s", port)
	params := netdicom.ServiceProviderParams{}
	callbacks := netdicom.ServiceProviderCallbacks{
		CEcho:  onCEchoRequest,
		CFind:  onCFindRequest,
		CStore: onCStoreRequest,
	}
	su := netdicom.NewServiceProvider(params, callbacks)
	err := su.Run(port)
	if err != nil {
		panic(err)
	}
}
