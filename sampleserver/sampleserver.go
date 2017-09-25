package main

// A simple PACS server.
//
// Usage: ./sampleserver -dir <directory> -port 11111
//
// It starts a DICOM server that serves files under <directory>.

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/yasushi-saito/go-dicom"
	"github.com/yasushi-saito/go-dicom/dicomio"
	"github.com/yasushi-saito/go-dicom/dicomuid"
	"github.com/yasushi-saito/go-netdicom"
	"github.com/yasushi-saito/go-netdicom/dimse"
	"v.io/x/lib/vlog"
)

var (
	portFlag = flag.String("port", "10000", "TCP port to listen to")
	dirFlag  = flag.String("dir", ".", `
The directory to locate DICOM files to report in C-FIND, C-MOVE, etc.
Files are searched recursivsely under this directory.
Defaults to '.'.`)
	outputFlag = flag.String("output", "", `
The directory to store files received by C-STORE.
If empty, use <dir>/incoming, where <dir> is the value of the -dir flag.`)
)

var pathSeq int32

type server struct {
	// Set of dicom files the server manages. Keys are file paths.
	mu       *sync.Mutex
	datasets map[string]*dicom.DataSet // guarded by mu.
}

func (ss *server) onCEchoRequest() dimse.Status {
	vlog.Info("Received C-ECHO")
	return dimse.Success
}

func (ss *server) onCStoreRequest(
	transferSyntaxUID string,
	sopClassUID string,
	sopInstanceUID string,
	data []byte) dimse.Status {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	path := path.Join(*outputFlag, fmt.Sprintf("image%04d.dcm", atomic.AddInt32(&pathSeq, 1)))

	vlog.Infof("Writing %s", path)
	e := dicomio.NewBytesEncoder(binary.LittleEndian, dicomio.ExplicitVR)
	dicom.WriteFileHeader(e,
		[]*dicom.Element{
			dicom.NewElement(dicom.TagTransferSyntaxUID, transferSyntaxUID),
			dicom.NewElement(dicom.TagMediaStorageSOPClassUID, sopClassUID),
			dicom.NewElement(dicom.TagMediaStorageSOPInstanceUID, sopInstanceUID),
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

	// Register the new file in ss.datasets.
	ds, err := dicom.ReadDataSetFromFile(path, dicom.ReadOptions{DropPixelData: true})
	if err != nil {
		vlog.Errorf("%s: failed to parse dicom file: %v", path, err)
	} else {
		ss.datasets[path] = ds
	}
	return dimse.Success
}

func (ss *server) onCFindRequest(
	transferSyntaxUID string,
	sopClassUID string,
	filters []*dicom.Element) chan netdicom.CFindResult {
	vlog.Infof("CFind: transfersyntax: %v, classuid: %v",
		dicomuid.UIDString(transferSyntaxUID),
		dicomuid.UIDString(sopClassUID))
	for _, filter := range filters {
		vlog.Infof("CFind: filter %v", filter)
	}
	ch := make(chan netdicom.CFindResult)

	// Match the filter against every file. This is just for demonstration.
	ss.mu.Lock()
	defer ss.mu.Unlock()
	for _, ds := range ss.datasets {
		var resp netdicom.CFindResult
		for _, filter := range filters {
			// TODO(saito): match the condition! This code returns every file in the database.
			elem, err := ds.LookupElementByTag(filter.Tag)
			if err == nil {
				resp.Elements = append(resp.Elements, elem)
			} else {
				resp.Elements = append(resp.Elements, dicom.NewElement(filter.Tag))
			}
		}
		ch <- resp
	}
	close(ch)
	return ch
}

// Find DICOM files in or under "dir" and read its attributes. The return value
// is a map from a pathname to dicom.Dataset (excluding PixelData).
func listDicomFiles(dir string) (map[string]*dicom.DataSet, error) {
	datasets := make(map[string]*dicom.DataSet)
	readFile := func(path string) {
		if _, ok := datasets[path]; ok {
			return
		}
		ds, err := dicom.ReadDataSetFromFile(path, dicom.ReadOptions{DropPixelData: true})
		if err != nil {
			vlog.Errorf("%s: failed to parse dicom file: %v", path, err)
			return
		}
		vlog.Infof("%s: read dicom file", path)
		datasets[path] = ds
	}
	walkCallback := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			vlog.Errorf("%v: skip file: %v", path, err)
			return nil
		}
		if (info.Mode() & os.ModeDir) != 0 {
			// If a directory contains file "DICOMDIR", all the files in the directory are DICOM files.
			if _, err := os.Stat(filepath.Join(path, "DICOMDIR")); err != nil {
				return nil
			}
			subpaths, err := filepath.Glob(path + "/*")
			if err != nil {
				vlog.Errorf("%v: glob: %v", path, err)
				return nil
			}
			for _, subpath := range subpaths {
				if !strings.HasSuffix(subpath, "DICOMDIR") {
					readFile(subpath)
				}
			}
			return nil
		}
		if strings.HasSuffix(path, ".dcm") {
			readFile(path)
		}
		return nil
	}
	if err := filepath.Walk(dir, walkCallback); err != nil {
		return nil, err
	}
	return datasets, nil
}

func main() {
	flag.Parse()
	vlog.ConfigureLibraryLoggerFromFlags()
	port := *portFlag
	if !strings.Contains(port, ":") {
		port = ":" + port
	}
	if *outputFlag == "" {
		*outputFlag = filepath.Join(*dirFlag, "incoming")
	}

	datasets, err := listDicomFiles(*dirFlag)
	if err != nil {
		vlog.Fatalf("%s: Failed to list dicom files: %v", *dirFlag, err)
	}
	ss := server{
		mu:       &sync.Mutex{},
		datasets: datasets,
	}
	vlog.Infof("Listening on %s", port)
	params := netdicom.ServiceProviderParams{}
	callbacks := netdicom.ServiceProviderCallbacks{
		CEcho: func() dimse.Status { return ss.onCEchoRequest() },
		CFind: func(transferSyntaxUID string, sopClassUID string, filter []*dicom.Element) chan netdicom.CFindResult {
			return ss.onCFindRequest(transferSyntaxUID, sopClassUID, filter)
		},
		CStore: func(transferSyntaxUID string,
			sopClassUID string,
			sopInstanceUID string,
			data []byte) dimse.Status {
			return ss.onCStoreRequest(transferSyntaxUID, sopClassUID, sopInstanceUID, data)
		},
	}
	sp := netdicom.NewServiceProvider(params, callbacks)
	err = sp.Run(port)
	if err != nil {
		panic(err)
	}
}
