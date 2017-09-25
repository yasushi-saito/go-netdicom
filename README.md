Golang implementation of DICOM network protocol.

See storeclient and storeserver for example.

Inspired by https://github.com/pydicom/pynetdicom3.

Status as of 2017-09-20:

- C-STORE works, both for the client and the server. Look at sampleclient and
  sampleserver for examples.  The server accepts C-STORE requests from a remote
  user and stores dcm files.  The client sends a file to a remote server using
  C-STORE.  I used pynetdicom3 storecu and storecp as peers.

- C-FIND sort of works, but it is fully functional nor
  tested. sampleserver/sampleclient contain code to exercise C-FIND.

- Compatibility has been tested against pynetdicom and Osirix MD.

TODO:

- Implement the rest of DIMSE protocols, such as C-MOVE, C-GET, etc.
- Better message validation.
- Documentation.
- Remove the "limit" param from the Decoder, and rely on io.EOF detection instead.
