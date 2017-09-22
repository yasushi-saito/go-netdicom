Golang implementation of DICOM network protocol.

See storeclient and storeserver for example.

Inspired by https://github.com/pydicom/pynetdicom3.

Status as of 2017-09-20:

- storeserver and storeclient work. The server accepts C-STORE requests
  from a remote user and stores dcm files.  The client sends a file to a remote
  server using C-STORE.  I used pynetdicom3 storecu and storecp as peers.

- C-FIND sort of works, but it is not well tested. storageserver/storageclient
  contains code to exercise C-FIND.

- Compatibility has been tested against pynetdicom and Osirix MD.

TODO:

- Implement the rest of DIMSE protocols, such as C-MOVE, C-GET, etc.
- Better message validation.
- Remove the "limit" param from the Decoder, and rely on io.EOF detection instead.
