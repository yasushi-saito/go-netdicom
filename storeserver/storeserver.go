package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"sync/atomic"

	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-dicom/dicomuid"
	"github.com/yasushi-saito/go-dicom/dicomio"
	"github.com/yasushi-saito/go-netdicom"
	"github.com/yasushi-saito/go-netdicom/dimse"
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
	e := dicomio.NewBytesEncoder(binary.LittleEndian, dicomio.ExplicitVR)
	dicom.WriteFileHeader(e,
		[]dicom.Element{
			*dicom.NewElement(dicom.TagTransferSyntaxUID, transferSyntaxUID),
			*dicom.NewElement(dicom.TagMediaStorageSOPClassUID, sopClassUID),
			*dicom.NewElement(dicom.TagMediaStorageSOPInstanceUID, sopInstanceUID),
		})
	e.WriteBytes(data)
	if err := e.Error(); err != nil {
		vlog.Errorf("%s: failed to write: %v", path, err)
		return dimse.Status{Status: dimse.StatusNotAuthorized}
	}
	bytes := e.Bytes()
	err := ioutil.WriteFile(path, bytes, 0644)
	if err != nil {
		vlog.Errorf("%s: %s", path, err)
		return dimse.Status{Status: dimse.StatusNotAuthorized}
	}
	return dimse.Success
}

func onCFindRequest(transferSyntaxUID string,
	sopClassUID string,
	data []byte) dimse.Status {
	decoder := dicomio.NewBytesDecoderWithTransferSyntax(data, transferSyntaxUID)
	var elems []*dicom.Element
	vlog.Infof("CFind: transfersyntax: %v, classuid: %v",
		dicomuid.UIDString(transferSyntaxUID),
		dicomuid.UIDString(sopClassUID))
	for decoder.Len() > 0 {
		elem := dicom.ReadDataElement(decoder)
		if decoder.Error() != nil {
			break
		}
		vlog.Infof("CFind: param: %v", elem)
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
