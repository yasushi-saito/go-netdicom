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

func onCEchoRequest() uint16 {
	vlog.Info("Received C-ECHO")
	return 0
}

func onCStoreRequest(
	transferSyntaxUID string,
	sopClassUID string,
	sopInstanceUID string,
	data []byte) uint16 {
	path := path.Join(*outputFlag, fmt.Sprintf("image%04d.dcm", atomic.AddInt32(&pathSeq, 1)))

	vlog.Infof("Writing %s", path)
	e := dicomio.NewEncoder(binary.LittleEndian, dicomio.ExplicitVR)
	dicom.WriteFileHeader(e, transferSyntaxUID, sopClassUID, sopInstanceUID)
	e.WriteBytes(data)
	bytes, err := e.Finish()

	if err != nil {
		vlog.Errorf("%s: failed to write: %v", path, err)
		return dimse.CStoreStatusOutOfResources
	}
	err = ioutil.WriteFile(path, bytes, 0644)
	if err != nil {
		vlog.Errorf("%s: %s", path, err)
		return dimse.CStoreStatusOutOfResources
	}
	return 0 // Success
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
		CStore: onCStoreRequest,
	}
	su := netdicom.NewServiceProvider(params, callbacks)
	err := su.Run(port)
	if err != nil {
		panic(err)
	}
}
