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
	"regexp"
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
	portFlag     = flag.String("port", "10000", "TCP port to listen to")
	aeFlag = flag.String("ae", "bogusae", "AE title of this server")
	remoteAEFlag = flag.String("remote-ae", "GBMAC0261:localhost:11112", `
Comma-separated list of remote AEs, in form aetitle:host:port, For example -remote-ae testae:foo.example.com:12345,testae2:bar.example.com:23456.
In this example, a C-GET or C-MOVE request to application entity "testae" will resolve to foo.example.com:12345.`)
	dirFlag = flag.String("dir", ".", `
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

func (ss *server) onCStore(
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
			dicom.MustNewElement(dicom.TagTransferSyntaxUID, transferSyntaxUID),
			dicom.MustNewElement(dicom.TagMediaStorageSOPClassUID, sopClassUID),
			dicom.MustNewElement(dicom.TagMediaStorageSOPInstanceUID, sopInstanceUID),
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

type filterMatch struct {
	path  string           // DICOM path name
	ds    *dicom.DataSet   // Contents of "path".
	elems []*dicom.Element // Elements that matched the filter
}

func (ss *server) findMatchingFiles(filters []*dicom.Element) ([]filterMatch, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	var matches []filterMatch
	for path, ds := range ss.datasets {
		allMatched := true
		match := filterMatch{path: path, ds: ds}
		for _, filter := range filters {
			ok, elem, err := dicom.Query(ds, filter)
			if err != nil {
				return matches, err
			}
			if !ok {
				vlog.VI(2).Infof("DS: %s: filter %v missed", path, filter)
				allMatched = false
				break
			}
			if elem != nil {
				match.elems = append(match.elems, elem)
			} else {
				elem, err := dicom.NewElement(filter.Tag)
				if err != nil {
					vlog.Error(err)
					return matches, err
				}
				match.elems = append(match.elems, elem)
			}
		}
		if allMatched {
			if len(match.elems) == 0 {
				panic(match)
			}
			matches = append(matches, match)
		}
	}
	return matches, nil
}

func (ss *server) onCFind(
	transferSyntaxUID string,
	sopClassUID string,
	filters []*dicom.Element) chan netdicom.CFindResult {
	for _, filter := range filters {
		vlog.Infof("CFind: filter %v", filter)
	}
	ch := make(chan netdicom.CFindResult, 128)
	vlog.Infof("CFind: transfersyntax: %v, classuid: %v",
		dicomuid.UIDString(transferSyntaxUID),
		dicomuid.UIDString(sopClassUID))

	// Match the filter against every file. This is just for demonstration
	go func() {
		matches, err := ss.findMatchingFiles(filters)
		vlog.Infof("C-FIND: found %d matches, err %v", len(matches), err)
		if err != nil {
			ch <- netdicom.CFindResult{Err: err}
		} else {
			for _, match := range matches {
				vlog.VI(1).Infof("C-FIND resp %s: %v", match.path, match.elems)
				ch <- netdicom.CFindResult{Elements: match.elems}
			}
		}
		close(ch)
	}()
	return ch
}

func (ss *server) onCMove(
	transferSyntaxUID string,
	sopClassUID string,
	filters []*dicom.Element) chan netdicom.CMoveResult {
	vlog.Infof("C-MOVE: transfersyntax: %v, classuid: %v",
		dicomuid.UIDString(transferSyntaxUID),
		dicomuid.UIDString(sopClassUID))
	for _, filter := range filters {
		vlog.Infof("C-MOVE: filter %v", filter)
	}
	ch := make(chan netdicom.CMoveResult, 128)
	go func() {
		matches, err := ss.findMatchingFiles(filters)
		vlog.Infof("C-MOVE: found %d matches, err %v", len(matches), err)
		if err != nil {
			ch <- netdicom.CMoveResult{Err: err}
		} else {
			for i, match := range matches {
				vlog.VI(1).Infof("C-MOVE resp %d %s: %v", i, match.path, match.elems)
				ch <- netdicom.CMoveResult{
					Remaining: len(matches) - i - 1,
					Path:      match.path,
					DataSet:   match.ds,
				}
			}
		}
		close(ch)
	}()
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

func parseRemoteAEFlag(flag string) (map[string]string, error) {
	aeMap := make(map[string]string)
	re := regexp.MustCompile("^([^:]+):(.+)$")
	for _, str := range strings.Split(flag, ",") {
		if str == "" {
			continue
		}
		m := re.FindStringSubmatch(str)
		if m == nil {
			return aeMap, fmt.Errorf("Failed to parse AE spec '%v'", str)
		}
		vlog.VI(1).Infof("Remote AE '%v' -> '%v'", m[1], m[2])
		aeMap[m[1]] = m[2]
	}
	return aeMap, nil
}

func canonicalizeHostPort(addr string) string {
	if !strings.Contains(addr, ":") {
		return ":" + addr
	}
	return addr
}

func main() {
	flag.Parse()
	vlog.ConfigureLibraryLoggerFromFlags()
	port := canonicalizeHostPort(*portFlag)
	if *outputFlag == "" {
		*outputFlag = filepath.Join(*dirFlag, "incoming")
	}

	remoteAEs, err := parseRemoteAEFlag(*remoteAEFlag)
	if err != nil {
		vlog.Fatalf("Failed to parse -remote-ae flag: %v", err)
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
	params := netdicom.ServiceProviderParams{
		AETitle: *aeFlag,
		RemoteAEs: remoteAEs,
	}
	callbacks := netdicom.ServiceProviderCallbacks{
		CEcho: func() dimse.Status {
			vlog.Info("Received C-ECHO")
			return dimse.Success
		},
		CFind: func(transferSyntaxUID string, sopClassUID string, filter []*dicom.Element) chan netdicom.CFindResult {
			return ss.onCFind(transferSyntaxUID, sopClassUID, filter)
		},
		CMove: func(transferSyntaxUID string, sopClassUID string, filter []*dicom.Element) chan netdicom.CMoveResult {
			return ss.onCMove(transferSyntaxUID, sopClassUID, filter)
		},
		CStore: func(transferSyntaxUID string,
			sopClassUID string,
			sopInstanceUID string,
			data []byte) dimse.Status {
			return ss.onCStore(transferSyntaxUID, sopClassUID, sopInstanceUID, data)
		},
	}
	sp := netdicom.NewServiceProvider(params, callbacks)
	err = sp.Run(port)
	if err != nil {
		panic(err)
	}
}
